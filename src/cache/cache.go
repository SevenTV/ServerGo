package cache

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/SevenTV/ServerGo/src/cache/decoder"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/bsm/redislock"
	"github.com/davecgh/go-spew/spew"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var json = jsoniter.Config{
	TagKey:                 "bson",
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

func genSha(prefix, collection string, q interface{}, opts interface{}) (string, error) {
	h := sha1.New()
	bytes, err := json.Marshal(q)
	if err != nil {
		return "", err
	}
	h.Write(utils.S2B(prefix))
	h.Write(utils.S2B(collection))
	h.Write(bytes)
	if opts != nil {
		bytes, err = json.Marshal(opts)
		if err != nil {
			return "", err
		}
		h.Write(bytes)
	}
	sha1 := hex.EncodeToString(h.Sum(nil))
	return sha1, nil
}

func query(ctx context.Context, collection, sha1 string, output interface{}) ([]primitive.ObjectID, error) {
	d, err := redis.GetCache(ctx, collection, sha1)
	if err != nil {
		return nil, err
	}

	items, ok := d[0].(string)
	if !ok {
		log.WithField("resp", spew.Sdump(d)).Error("redis bad response, expected string")
		return nil, redis.ErrNil
	}
	missingItems, ok := d[1].([]interface{})
	if !ok {
		log.WithField("resp", spew.Sdump(d)).Error("redis bad response, expected array")
		return nil, redis.ErrNil
	}

	if err = json.UnmarshalFromString(items, output); err != nil {
		return nil, err
	}

	mItems := make([]primitive.ObjectID, len(missingItems))
	for i, v := range missingItems {
		s, ok := v.(string)
		if !ok {
			log.WithField("resp", spew.Sdump(d)).Error("redis bad response, expected string")
			return nil, redis.ErrNil
		}
		if mItems[i], err = primitive.ObjectIDFromHex(s); err != nil {
			return nil, err
		}
	}

	return mItems, nil
}

func Find(ctx context.Context, collection, commonIndex string, q interface{}, output interface{}, opts ...*options.FindOptions) error {
	if !utils.IsSliceArrayPointer(output) {
		return fmt.Errorf("the output must be a pointer to some array")
	}

	sha1, err := genSha("find", collection, q, opts)
	if err != nil {
		return err
	}

	val := []bson.M{}

	missingIDs, err := query(ctx, collection, sha1, &val)
	if err != nil {
		if err != redis.ErrNil {
			log.WithError(err).Error("redis")
		}
		// MongoQuery
		cur, err := mongo.Database.Collection(collection).Find(ctx, q, opts...)
		if err != nil {
			return err
		}

		out := []bson.M{}

		if err = cur.All(ctx, &out); err != nil {
			return err
		}

		args := make([]string, len(out)*2)
		for i, v := range out {
			oid, ok := v["_id"].(primitive.ObjectID)
			if !ok {
				log.WithField("data", spew.Sdump(out)).Error("invalid resp mongo")
				return fmt.Errorf("invalid resp mongo")
			}
			args[2*i] = oid.Hex()
			args[2*i+1], err = json.MarshalToString(v)
			if err != nil {
				return err
			}
		}
		_, err = redis.SetCache(ctx, collection, sha1, commonIndex, args...)
		if err != nil {
			return err
		}

		return decoder.Decode(out, output)
	} else if len(missingIDs) > 0 {
		cur, err := mongo.Database.Collection(collection).Find(ctx, bson.M{
			"_id": bson.M{
				"$in": missingIDs,
			},
		})
		if err != nil {
			return err
		}

		results := []bson.M{}
		if err = cur.All(ctx, &results); err != nil {
			return err
		}

		args := make([]string, len(results)*2)
		for i, v := range results {
			oid, ok := v["_id"].(primitive.ObjectID)
			if !ok {
				log.WithField("data", spew.Sdump(results)).Error("invalid resp mongo")
				return fmt.Errorf("invalid resp mongo")
			}
			args[2*i] = oid.Hex()
			args[2*i+1], err = json.MarshalToString(v)
			if err != nil {
				return err
			}
		}
		_, err = redis.SetCache(ctx, collection, sha1, commonIndex, args...)
		if err != nil {
			return err
		}

		if err = decoder.Decode(results, output); err != nil {
			return err
		}
	}

	return decoder.Decode(val, output)
}

func FindOne(ctx context.Context, collection, commonIndex string, q interface{}, output interface{}, opts ...*options.FindOneOptions) error {
	if !utils.IsPointer(output) {
		return fmt.Errorf("the output must be a pointer")
	}

	sha1, err := genSha("find-one", collection, q, opts)
	if err != nil {
		return err
	}

	val := []bson.M{}
	_, err = query(ctx, collection, sha1, &val)
	if err != nil {
		if err != redis.ErrNil {
			log.WithError(err).Error("redis")
		}
		// MongoQuery
		res := mongo.Database.Collection(collection).FindOne(ctx, q, opts...)
		err = res.Err()
		if err != nil {
			return err
		}

		out := bson.M{}
		if err = res.Decode(&out); err != nil {
			return err
		}

		oid, ok := out["_id"].(primitive.ObjectID)
		if !ok {
			log.WithField("data", spew.Sdump(out)).Error("invalid resp mongo")
			return fmt.Errorf("invalid resp mongo")
		}

		data, err := json.MarshalToString(out)
		if err != nil {
			return err
		}
		_, err = redis.SetCache(ctx, collection, sha1, commonIndex, oid.Hex(), data)
		if err != nil {
			return err
		}

		return decoder.Decode(out, output)
	}
	return decoder.Decode(val[0], output)
}

// Gets the collection size then caches it in redis for some time
func GetCollectionSize(ctx context.Context, collection string, q interface{}, opts ...*options.CountOptions) (int64, error) {
	sha1, err := genSha("collection-size", collection, q, opts)
	if err != nil {
		return 0, err
	}
	key := "cached:collection-size:" + collection + ":" + sha1

	if count, err := redis.Client.Get(ctx, key).Int64(); err == nil { // Try to find the cached value in redis
		return count, nil
	} else { // Otherwise, query mongo
		count, err := mongo.Database.Collection(collection).CountDocuments(ctx, q, opts...)
		if err != nil {
			return 0, err
		}

		redis.Client.Set(ctx, key, count, 1*time.Minute)
		return count, nil
	}

}

// Send a GET request to an endpoint and cache the result
func CacheGetRequest(ctx context.Context, uri string, cacheDuration time.Duration, errorCacheDuration time.Duration, headers ...struct {
	Key   string
	Value string
}) (*cachedGetRequest, error) {
	//
	encodedURI := base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(uri)))
	h := sha1.New()
	h.Write(utils.S2B(encodedURI))
	sha1 := hex.EncodeToString(h.Sum(nil))
	key := "cached:http-get:" + sha1

	// Establish distributed lock
	// This prevents the same request from being executed multiple times simultaneously
	lock, err := redis.GetLocker().Obtain(ctx, "lock:http-get"+sha1, 10*time.Second, &redislock.Options{
		RetryStrategy: redislock.ExponentialBackoff(4, 750),
	})
	if err != nil {
		log.WithError(err).Error("CacheGetRequest")
		return nil, err
	}
	defer func() {
		_ = lock.Release(context.Background())
	}()

	// Try to find the cached result of this request
	cachedBody := redis.Client.Get(ctx, key).Val()
	if cachedBody != "" {
		return &cachedGetRequest{
			Status:     "OK",
			StatusCode: 200,
			Body:       utils.S2B(cachedBody),
			FromCache:  true,
		}, nil
	}

	// If not cached let's make the request
	req, _ := http.NewRequest("GET", uri, nil)
	for _, header := range headers { // Add custom headers
		req.Header.Add(header.Key, header.Value)
	}

	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"status_code":    resp.StatusCode,
		"status":         resp.Status,
		"response_in_ms": time.Since(startedAt).Milliseconds(),
		"completed_at":   time.Now(),
	}).Info("CacheGetRequest")

	// Read the body as byte slice
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Cache the request body
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		redis.Client.Set(ctx, key, body, cacheDuration)
	} else if errorCacheDuration > 0 { // Cache as errored for specified amount of time?
		redis.Client.Set(ctx, key, fmt.Sprintf("err=%v", resp.StatusCode), errorCacheDuration)
	}

	// Return request
	return &cachedGetRequest{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
		FromCache:  false,
	}, nil
}

type cachedGetRequest struct {
	Status     string
	StatusCode int
	Header     map[string][]string
	Body       []byte
	FromCache  bool
}
