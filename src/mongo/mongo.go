package mongo

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
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

	_, err = Collection(CollectionNameEmotes).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"name": 1}},
		{Keys: bson.M{"owner": 1}},
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

	_, err = Collection(CollectionNameUsers).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"id": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"login": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"role": 1}},
		{Keys: bson.M{"editors": 1}},
		{Keys: bson.M{"emotes": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Collection(CollectionNameBans).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"user_id": 1}},
		{Keys: bson.M{"issued_by": 1}},
		{Keys: bson.M{"active": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Collection(CollectionNameAudit).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"type": 1}},
		{Keys: bson.M{"target.type": 1}},
		{Keys: bson.M{"target.id": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Collection(CollectionNameReports).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"reporter_id": 1}},
		{Keys: bson.M{"target.type": 1}},
		{Keys: bson.M{"target.id": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_, err = Collection(CollectionNameBadges).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"name": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

	_ = Database.CreateCollection(ctx, "notifications")
	_, err = Collection(CollectionNameNotificationsRead).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.M{"target": 1}},
		{Keys: bson.M{"notification": 1}},
	})
	if err != nil {
		log.WithError(err).Fatal("mongo")
	}

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

func Collection(name CollectionName) *mongo.Collection {
	return Database.Collection(string(name))
}

type CollectionName string

var (
	CollectionNameEmotes            = CollectionName("emotes")
	CollectionNameUsers             = CollectionName("users")
	CollectionNameBans              = CollectionName("bans")
	CollectionNameReports           = CollectionName("reports")
	CollectionNameBadges            = CollectionName("badges")
	CollectionNameRoles             = CollectionName("roles")
	CollectionNameAudit             = CollectionName("audit")
	CollectionNameEntitlements      = CollectionName("entitlements")
	CollectionNameNotifications     = CollectionName("notifications")
	CollectionNameNotificationsRead = CollectionName("notifications_read")
)

func HexIDSliceToObjectID(arr []string) []primitive.ObjectID {
	r := make([]primitive.ObjectID, len(arr))
	for i, s := range arr {
		if id, err := primitive.ObjectIDFromHex(s); err == nil {
			r[i] = id
		}
	}

	return r
}
