package actions

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MergeEmote: Merge an emote into another emote, transferring all its channels and swapping aliases
func (*emotes) MergeEmote(ctx context.Context, opts MergeEmoteOptions) (*datastructure.Emote, error) {
	// The old & new ID can't be equal
	if opts.OldID == opts.NewID {
		return nil, fmt.Errorf("Cannot merge emote into itself")
	}

	logInfo := log.WithFields(log.Fields{
		"OldEmoteID": opts.OldID,
		"NewEmoteID": opts.NewID,
		"ActorID":    opts.Actor.ID,
	})
	logInfo.Info("Starting Emote Merge")

	// Get the old & new emotes
	var (
		oldEmote datastructure.Emote
		newEmote datastructure.Emote
	)
	res := mongo.Collection(mongo.CollectionNameEmotes).FindOne(ctx, bson.M{
		"_id": opts.OldID,
	})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&oldEmote); err != nil {
		return nil, err
	}
	res = mongo.Collection(mongo.CollectionNameEmotes).FindOne(ctx, bson.M{
		"_id": opts.NewID,
	})
	if err := res.Decode(&newEmote); err != nil {
		return nil, err
	}

	switchedChannels := []primitive.ObjectID{}
	userOps := []mongo.WriteModel{}
	{
		var channels []*datastructure.User

		// Fetch all users with the emote enabled
		cur, err := mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
			"emotes": bson.M{
				"$in": []primitive.ObjectID{oldEmote.ID},
			},
		})
		if err != nil {
			return nil, err
		}
		if err := cur.All(ctx, &channels); err != nil {
			return nil, err
		}

		// Find aliases
		for _, ch := range channels {
			update := bson.M{
				"emotes.$[filter]": newEmote.ID,
			}

			// Transfer Aliases
			//
			// If an alias was set, it is transferred to the new emote
			// If no alias was set but the old emote name is not equal to the new, an alias for the new emote is created with the old emote's name
			a, ok := ch.EmoteAlias[oldEmote.ID.Hex()]
			if ok || oldEmote.Name != newEmote.Name {
				if ch.EmoteAlias == nil {
					ch.EmoteAlias = map[string]string{}
				}

				if ok { // The user had an alias for the old emote
					ch.EmoteAlias[newEmote.ID.Hex()] = a     // Set new alias
					delete(ch.EmoteAlias, oldEmote.ID.Hex()) // Delete old alias

				} else if oldEmote.Name != newEmote.Name { // Old emote name is different from the emote it is being merged to
					ch.EmoteAlias[newEmote.ID.Hex()] = oldEmote.Name
				}

				update["emote_alias"] = ch.EmoteAlias
			}

			// Append a bulk write update operation.
			// This will be executed later once the merge is confirmed
			switchedChannels = append(switchedChannels, ch.ID)
			userOps = append(
				userOps, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": ch.ID}).
					SetUpdate(bson.M{
						"$set": update,
					}).SetArrayFilters(options.ArrayFilters{
					Filters: []interface{}{
						bson.M{"filter": oldEmote.ID},
					},
				}),
			)
		}
	}

	// Update the users
	if len(userOps) > 0 {
		result, err := mongo.Collection(mongo.CollectionNameUsers).BulkWrite(ctx, userOps)
		if err != nil {
			log.WithError(err).WithField("count", len(userOps)).Error("mongo, failed to update users during emote merger")
			return nil, err
		}
		logInfo.Infof("Targeted %d users and updated %d users during merger of Emote(id=%v) into Emote(id=%v)",
			result.MatchedCount, result.ModifiedCount, oldEmote.ID.Hex(), newEmote.ID.Hex(),
		)
	} else {
		logInfo.Infof("Updated no users during merger of Emote(id=%v) into Emote(id=%v)", oldEmote.ID.Hex(), newEmote.ID.Hex())
	}

	// Send notifications
	{
		// Send a notification to the old emote's owner that their emote was merged
		go func() {
			if err := Notifications.Create().
				SetTitle("An Emote You Own Was Merged").
				AddTargetUsers(oldEmote.OwnerID).
				AddTextMessagePart("Your emote ").
				AddEmoteMentionPart(oldEmote.ID).
				AddTextMessagePart(" has been merged into ").
				AddUserMentionPart(newEmote.OwnerID).
				AddTextMessagePart("'s ").
				AddEmoteMentionPart(newEmote.ID).
				AddTextMessagePart(" by ").
				AddUserMentionPart(opts.Actor.ID).
				AddTextMessagePart(fmt.Sprintf(". Reason: \"%v\"", opts.Reason)).
				Write(context.Background()); err != nil {
				log.WithError(err).Error("failed to create notification")
			}
		}()

		// Send a notification to the channels affected
		go func() {
			if err := Notifications.Create().
				SetTitle("A Channel Emote Was Merged").
				AddTargetUsers(switchedChannels...).
				AddTextMessagePart("One of your active channel emotes, ").
				AddEmoteMentionPart(oldEmote.ID).
				AddTextMessagePart(" was merged into ").
				AddUserMentionPart(newEmote.OwnerID).
				AddTextMessagePart("'s ").
				AddEmoteMentionPart(newEmote.ID).
				AddTextMessagePart(" by ").
				AddUserMentionPart(opts.Actor.ID).
				AddTextMessagePart(fmt.Sprintf("for the reason \"%v\". No further action is required.", opts.Reason)).
				Write(context.Background()); err != nil {
				log.WithError(err).Error("failed to create notification")
			}
		}()

		// Send a notification to the owner of the new emote
		go func() {
			if err := Notifications.Create().
				SetTitle("An Emote Was Merged Into One You Own").AddTargetUsers(newEmote.OwnerID).
				AddTextMessagePart("The emote ").
				AddEmoteMentionPart(oldEmote.ID).
				AddTextMessagePart(", which was owned by ").
				AddUserMentionPart(oldEmote.OwnerID).
				AddTextMessagePart(" was merged into your emote ").
				AddEmoteMentionPart(newEmote.ID).
				AddTextMessagePart(" by ").
				AddUserMentionPart(opts.Actor.ID).
				AddTextMessagePart(fmt.Sprintf("for the reason \"%v\". %d new channels have been added to your emote and no further action is required.", opts.Reason, len(switchedChannels))).
				Write(context.Background()); err != nil {
				log.WithError(err).Error("failed to create notification")
			}
		}()

		// Send to Discord
		go discord.SendEmoteMerge(oldEmote, newEmote, *opts.Actor, int32(len(switchedChannels)), opts.Reason)
	}

	// Now we will delete the old emote
	if err := Emotes.Delete(ctx, &oldEmote); err != nil {
		return nil, err
	}

	// Create an Audit Log
	_, err := mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeEmoteMerge,
		CreatedBy: opts.Actor.ID,
		Target:    &datastructure.Target{ID: &oldEmote.ID, Type: "emotes"},
		Changes: []*datastructure.AuditLogChange{
			{Key: "merged_into", OldValue: nil, NewValue: newEmote.ID},
		},
		Reason: &opts.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	return &newEmote, nil
}

type MergeEmoteOptions struct {
	Actor  *datastructure.User
	OldID  primitive.ObjectID
	NewID  primitive.ObjectID
	Reason string
}
