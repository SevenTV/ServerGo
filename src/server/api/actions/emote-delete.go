package actions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

func (*emotes) Delete(ctx context.Context, emote *datastructure.Emote) error {
	_, err := mongo.Collection(mongo.CollectionNameEmotes).UpdateOne(ctx, bson.M{
		"_id": emote.ID,
	}, bson.M{
		"$set": bson.M{
			"status":             datastructure.EmoteStatusDeleted,
			"last_modified_date": time.Now(),
		},
	})
	if err != nil {
		log.WithError(err).Error("mongo")
		return err
	}

	wg := &sync.WaitGroup{}
	wg.Add(4)

	for i := 1; i <= 4; i++ {
		go func(i int) {
			defer wg.Done()
			obj := fmt.Sprintf("emote/%s", emote.ID.Hex())
			err := aws.Expire(configure.Config.GetString("aws_cdn_bucket"), obj, i)
			if err != nil {
				log.WithError(err).WithField("obj", obj).Error("aws")
			}
		}(i)
	}

	_, err = mongo.Collection(mongo.CollectionNameUsers).UpdateMany(ctx, bson.M{
		"emotes": emote.ID,
	}, bson.M{
		"$pull": bson.M{
			"emotes": emote.ID,
		},
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	wg.Wait()

	return nil
}
