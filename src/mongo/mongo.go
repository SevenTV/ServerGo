package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var json = jsoniter.Config{
// 	TagKey:                 "bson",
// 	EscapeHTML:             true,
// 	SortMapKeys:            true,
// 	ValidateJsonRawMessage: true,
// }.Froze()

var Database *mongo.Database

var ErrNoDocuments = mongo.ErrNoDocuments

type Pipeline = mongo.Pipeline
type WriteModel = mongo.WriteModel

var ChangeStreamChan = make(chan ChangeStreamEvent)

func NewUpdateOneModel() *mongo.UpdateOneModel {
	return mongo.NewUpdateOneModel()
}

func init() {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*25)
	defer cancel()

	clientOptions := options.Client().ApplyURI(configure.Config.GetString("mongo_uri"))
	if configure.Config.GetBool("mongo_direct") {
		clientOptions.SetDirect(true)
	}
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	Database = client.Database(configure.Config.GetString("mongo_db"))

	_, err = Database.Collection("emotes").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"name": 1}},
		{Keys: bson.M{"owner_id": 1}},
		{Keys: bson.M{"tags": 1}},
		{Keys: bson.M{"status": 1}},
		{Keys: bson.M{"last_modified_date": 1}, Options: options.Index().SetExpireAfterSeconds(int32(time.Hour * 24 * 21 / time.Second)).SetPartialFilterExpression(bson.M{
			"status": datastructure.EmoteStatusDeleted,
		})},
		{Keys: bson.M{"channel_count_checked_at": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Database.Collection("users").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"id": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"login": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"rank": 1}},
		{Keys: bson.M{"editors": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Database.Collection("bans").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"user_id": 1}},
		{Keys: bson.M{"issued_by": 1}},
		{Keys: bson.M{"active": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Database.Collection("audit").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"type": 1}},
		{Keys: bson.M{"target.type": 1}},
		{Keys: bson.M{"target.id": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Database.Collection("reports").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"reporter_id": 1}},
		{Keys: bson.M{"target.type": 1}},
		{Keys: bson.M{"target.id": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Database.Collection("badges").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"name": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Database.Collection("notifications").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"target_users": 1}},
		{Keys: bson.M{"target_roles": 1}},
		{Keys: bson.M{"content.title": 1}},
		{Keys: bson.M{"content.message_parts.mention": 1}},
	})
	_, err = Database.Collection("notifications_read").Indexes().CreateMany(ctx, []mongo.IndexModel{})

	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	for _, v := range []string{"users", "emotes", "bans", "reports", "audit"} {
		go func(col string) {
			ctx := context.TODO()
			userChangeStream, err := Database.Collection(col).Watch(ctx, mongo.Pipeline{}, opts)
			if err != nil {
				log.WithError(err).Fatal("mongo")
			}
			go func() {
				for userChangeStream.Next(ctx) {
					data := bson.M{}
					if err := userChangeStream.Decode(&data); err != nil {
						log.WithError(err).WithField("col", col).Error("mongo change stream")
						continue
					}
					changeStream(ctx, col, data)
				}
			}()
		}(v)
	}

}

func HexIDSliceToObjectID(arr []string) []primitive.ObjectID {
	r := make([]primitive.ObjectID, len(arr))
	for i, s := range arr {
		if id, err := primitive.ObjectIDFromHex(s); err == nil {
			r[i] = id
		}
	}

	return r
}

func changeStream(ctx context.Context, collection string, data bson.M) {
	defer func() {
		if err := recover(); err != nil {
			log.WithField("error", err).Error("mongo change stream")
		}
	}()
	// spew.Dump(data)

	// Send to channel
	var event ChangeStreamEvent
	if b, err := bson.Marshal(data); err == nil {
		_ = bson.Unmarshal(b, &event)

		// Send to subscribers
		for i := range changeSubscribers {
			subscriber := changeSubscribers[i]
			if subscriber.Collection != collection {
				continue
			}

			subscriber.Channel <- event
		}
	} else {
		log.WithError(err).Error("mongo change stream")
		return
	}

	var commonIndex string
	var ojson string
	eventType := data["operationType"].(string)
	eventID := (data["_id"].(bson.M))["_data"].(string)
	oid := ((data["documentKey"].(bson.M))["_id"].(primitive.ObjectID)).Hex()
	if eventType != "delete" {
		document := data["fullDocument"].(bson.M)
		dataString, err := jsoniter.MarshalToString(document)
		if err != nil {
			log.WithError(err).Error("mongo change stream")
			return
		}
		ojson = dataString
	}

	_, err := redis.InvalidateCache(ctx, fmt.Sprintf("cached:events:%s", eventID), collection, oid, commonIndex, ojson)
	if err != nil {
		log.WithError(err).Error("mongo change stream")
	}
}

var changeSubscribers = make(map[uuid.UUID]changeStreamSubscription)

func Subscribe(collection string, id uuid.UUID, ch chan ChangeStreamEvent) {
	changeSubscribers[id] = changeStreamSubscription{
		Collection: collection,
		Channel:    ch,
	}
}

func Unsubscribe(id uuid.UUID) {
	delete(changeSubscribers, id)
}

type changeStreamSubscription struct {
	Collection string
	Channel    chan ChangeStreamEvent
}

type ChangeStreamEvent struct {
	FullDocument  []byte                       `bson:"fullDocument"`
	Namespace     changeStreamEventNamespace   `bson:"ns"`
	OperationType string                       `bson:"operationType"`
	DocumentKey   changeStreamEventDocumentKey `bson:"documentKey"`
}

type changeStreamEventDocumentKey struct {
	ID string `bson:"_id"`
}

type changeStreamEventNamespace struct {
	Collection string `bson:"coll"`
	Database   string `bson:"db"`
}
