package cache

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/cache/decoder"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
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

var mutex sync.Mutex

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

func query(collection, sha1 string, output interface{}) ([]primitive.ObjectID, error) {
	d, err := redis.GetCache(collection, sha1)
	if err != nil {
		return nil, err
	}

	items, ok := d[0].(string)
	if !ok {
		log.Errorf("invalid redis, resp=%s", spew.Sdump(d))
		return nil, redis.ErrNil
	}
	missingItems, ok := d[1].([]interface{})
	if !ok {
		log.Errorf("invalid redis, resp=%s", spew.Sdump(d))
		return nil, redis.ErrNil
	}

	if err = json.UnmarshalFromString(items, output); err != nil {
		return nil, err
	}

	mItems := make([]primitive.ObjectID, len(missingItems))
	for i, v := range missingItems {
		s, ok := v.(string)
		if !ok {
			log.Errorf("invalid redis, resp=%s", spew.Sdump(d))
			return nil, redis.ErrNil
		}
		if mItems[i], err = primitive.ObjectIDFromHex(s); err != nil {
			return nil, err
		}
	}

	return mItems, nil
}

func Find(collection, commonIndex string, q interface{}, output interface{}, opts ...*options.FindOptions) error {
	if !utils.IsSliceArrayPointer(output) {
		return fmt.Errorf("the output must be a pointer to some array")
	}

	sha1, err := genSha("find", collection, q, opts)
	if err != nil {
		return err
	}

	val := []bson.M{}

	missingIDs, err := query(collection, sha1, &val)
	if err != nil {
		if err != redis.ErrNil {
			log.Errorf("redis, err=%v", err)
		}
		// MongoQuery
		cur, err := mongo.Database.Collection(collection).Find(mongo.Ctx, q, opts...)
		if err != nil {
			return err
		}

		out := []bson.M{}

		if err = cur.All(mongo.Ctx, &out); err != nil {
			return err
		}

		args := make([]string, len(out)*2)
		for i, v := range out {
			oid, ok := v["_id"].(primitive.ObjectID)
			if !ok {
				return fmt.Errorf("invalid mongo, resp=%s", spew.Sdump(out))
			}
			args[2*i] = oid.Hex()
			args[2*i+1], err = json.MarshalToString(v)
			if err != nil {
				return err
			}
		}
		_, err = redis.SetCache(collection, sha1, commonIndex, args...)
		if err != nil {
			return err
		}

		return decoder.Decode(out, output)
	} else if len(missingIDs) > 0 {
		cur, err := mongo.Database.Collection(collection).Find(mongo.Ctx, bson.M{
			"_id": bson.M{
				"$in": missingIDs,
			},
		})
		if err != nil {
			return err
		}

		results := []bson.M{}
		if err = cur.All(mongo.Ctx, &results); err != nil {
			return err
		}

		args := make([]string, len(results)*2)
		for i, v := range results {
			oid, ok := v["_id"].(primitive.ObjectID)
			if !ok {
				return fmt.Errorf("invalid mongo, resp=%s", spew.Sdump(results))
			}
			args[2*i] = oid.Hex()
			args[2*i+1], err = json.MarshalToString(v)
			if err != nil {
				return err
			}
		}
		_, err = redis.SetCache(collection, sha1, commonIndex, args...)
		if err != nil {
			return err
		}

		if err = decoder.Decode(results, output); err != nil {
			return err
		}
	}

	return decoder.Decode(val, output)
}

func FindOne(collection, commonIndex string, q interface{}, output interface{}, opts ...*options.FindOneOptions) error {
	if !utils.IsPointer(output) {
		return fmt.Errorf("the output must be a pointer")
	}

	sha1, err := genSha("find-one", collection, q, opts)
	if err != nil {
		return err
	}

	val := []bson.M{}
	_, err = query(collection, sha1, &val)
	if err != nil {
		if err != redis.ErrNil {
			log.Errorf("redis, err=%v", err)
		}
		// MongoQuery
		res := mongo.Database.Collection(collection).FindOne(mongo.Ctx, q, opts...)
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
			return fmt.Errorf("invalid mongo, resp=%s", spew.Sdump(out))
		}

		data, err := json.MarshalToString(out)
		if err != nil {
			return err
		}
		_, err = redis.SetCache(collection, sha1, commonIndex, oid.Hex(), data)
		if err != nil {
			return err
		}

		return decoder.Decode(out, output)
	}
	return decoder.Decode(val[0], output)
}

// Gets the collection size then caches it in redis for some time
func GetCollectionSize(collection string, q interface{}, opts ...*options.CountOptions) (int64, error) {
	sha1, err := genSha("collection-size", collection, q, opts)
	if err != nil {
		return 0, err
	}
	key := "cached:collection-size:" + collection + ":" + sha1

	if count, err := redis.Client.Get(redis.Ctx, key).Int64(); err == nil { // Try to find the cached value in redis
		return count, nil
	} else { // Otherwise, query mongo
		count, err := mongo.Database.Collection(collection).CountDocuments(mongo.Ctx, q, opts...)
		if err != nil {
			return 0, err
		}

		redis.Client.Set(redis.Ctx, key, count, 5*time.Minute)
		return count, nil
	}

}

// Send a GET request to an endpoint and cache the result
func CacheGetRequest(uri string, cacheDuration time.Duration, errorCacheDuration time.Duration, headers ...struct {
	Key   string
	Value string
}) (*cachedGetRequest, error) {
	mutex.Lock()
	defer mutex.Unlock()

	encodedURI := base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(uri)))
	h := sha1.New()
	h.Write(utils.S2B(encodedURI))
	sha1 := hex.EncodeToString(h.Sum(nil))

	key := "cached:http-get:" + sha1

	// Try to find the cached result of this request
	cachedBody := redis.Client.Get(redis.Ctx, key).Val()
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Read the body as byte slice
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Cache the request body
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		redis.Client.Set(redis.Ctx, key, body, cacheDuration)
	} else if errorCacheDuration > 0 { // Cache as errored for specified amount of time?
		redis.Client.Set(redis.Ctx, key, fmt.Sprintf("err=%v", resp.StatusCode), errorCacheDuration)
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
