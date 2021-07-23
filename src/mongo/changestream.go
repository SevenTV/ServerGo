package mongo

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
