package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/bsm/redislock"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Update the channel count of all emotes
func CheckEmotesPopularity(ctx context.Context) error {
	// Acquire lock. We won't allow any other pod to execute this concurrently
	lockCtx := context.Background()
	lock, err := redis.GetLocker().Obtain(lockCtx, "lock:task:check-emotes-popularity", time.Hour*6+time.Second*30, &redislock.Options{
		RetryStrategy: redislock.ExponentialBackoff(time.Second*5, time.Minute*10),
	})
	if err != nil {
		return err
	}

	// Create ticker
	// This is the interval between how often emote popularities should refresh
	ticker := time.NewTicker(6 * time.Hour)
	log.Info("Task=CheckEmotesPopularity, starting now")

	f := func() error {
		log.Info("Task=CheckEmotesPopularity, starting update...")
		wg := sync.WaitGroup{}
		wg.Add(1)
		defer wg.Done()
		go discord.SendPopularityCheckUpdateNotice(&wg)

		// Create a pipeline for ranking emotes by channel count
		popCheck := mongo.Pipeline{
			bson.D{
				bson.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "users",
						"localField":   "_id",
						"foreignField": "emotes",
						"as":           "channels",
					},
				},
			},
			bson.D{
				bson.E{
					Key: "$addFields",
					Value: bson.M{
						"channel_count":     "$channel_count",
						"channel_count_new": bson.M{"$size": "$channels"},
					},
				},
			},
			bson.D{
				bson.E{
					Key:   "$unset",
					Value: "channels",
				},
			},
		}
		cur, err := mongo.Collection(mongo.CollectionNameEmotes).Aggregate(ctx, popCheck)
		if err != nil {
			log.WithError(err).Error("mongo")
			return err
		}

		countedEmotes := []*channelCountUpdate{}
		if err := cur.All(ctx, &countedEmotes); err == nil && len(countedEmotes) > 0 {
			diffEmotes := []*channelCountUpdate{}
			for _, e := range countedEmotes {
				if e.Old == e.New {
					continue
				}

				diffEmotes = append(diffEmotes, e)
			}

			if err == nil { // Get the unchecked emotes, add them to a slice
				ops := make([]mongo.WriteModel, len(diffEmotes))
				if len(ops) == 0 {
					log.Info("Task=CheckEmotesPopularity, no change in emote popularities.")
					return nil
				}
				for i, e := range diffEmotes {
					now := time.Now()
					update := mongo.NewUpdateOneModel(). // Append update ops for bulk write
										SetFilter(bson.M{"_id": e.ID}).
										SetUpdate(bson.M{
							"$set": bson.M{
								"channel_count":            e.New,
								"channel_count_checked_at": &now,
							},
						})

					ops[i] = update
				}

				// Update unchecked with channel count data
				_, err := mongo.Collection(mongo.CollectionNameEmotes).BulkWrite(ctx, ops)
				if err != nil {
					log.WithError(err).WithField("count", len(countedEmotes)).Error("mongo was unable to update channel count emotes")
				}
			}
		}
		log.WithError(err).WithField("count", len(countedEmotes)).Error("mongo was unable to update channel count emotes")

		return err
	}

	defer func() {
		log.Info("Task=CheckEmotesPopularity, giving up lock, another pod will take over.")
		if err := lock.Release(lockCtx); err != nil {
			log.WithError(err).Error("CheckEmotesPopularity, failed to release lock")
		}

		ticker.Stop()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Refresh lock
				if err := lock.Refresh(ctx, time.Hour*6, &redislock.Options{}); err != nil {
					log.WithError(err).Error("CheckEmotesPopularity, could not refresh lock")
				}

				// Run the check
				go func() {
					err := f()
					if err != nil {
						log.WithError(err).Error("CheckEmotesPopularity")
						ticker.Stop()
					}

					log.Info("Task=CheckEmotesPopularity, completed update cycle! Keeping lock.")
				}()
			}
		}
	}()

	<-ctx.Done()
	return nil
}

type channelCountUpdate struct {
	ID  primitive.ObjectID `json:"_id" bson:"_id"`
	Old int32              `json:"channel_count" bson:"channel_count"`
	New int32              `json:"channel_count_new" bson:"channel_count_new"`
}
