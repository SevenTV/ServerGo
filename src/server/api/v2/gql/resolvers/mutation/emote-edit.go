package mutation_resolvers

import (
	"context"
	"time"

	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/SevenTV/ServerGo/src/validation"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// Mutate Emote - Edit
//
func (*MutationResolver) EditEmote(ctx context.Context, args struct {
	Emote  emoteInput
	Reason *string
}) (*query_resolvers.EmoteResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	update := bson.M{}
	logChanges := []*datastructure.AuditLogChange{}
	req := args.Emote

	if req.Name != nil {
		if !validation.ValidateEmoteName(utils.S2B(*req.Name)) {
			return nil, resolvers.ErrInvalidName
		}
		update["name"] = *req.Name
	}
	if req.OwnerID != nil {
		id, err := primitive.ObjectIDFromHex(*req.OwnerID)
		if err != nil {
			return nil, resolvers.ErrInvalidOwner
		}
		update["owner"] = id
	}
	if req.Tags != nil {
		tags := *req.Tags
		if len(tags) > 6 {
			return nil, resolvers.ErrInvalidTags
		}
		if ok, _ := validation.ValidateEmoteTags(tags); !ok {
			return nil, resolvers.ErrInvalidTag
		}
		update["tags"] = tags
	}
	if req.Visibility != nil {
		i32 := int32(*req.Visibility)

		update["visibility"] = i32
	}

	if len(update) == 0 {
		return nil, resolvers.ErrInvalidUpdate
	}

	id, err := primitive.ObjectIDFromHex(req.ID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	res := mongo.Collection(mongo.CollectionNameEmotes).FindOne(ctx, bson.M{
		"_id": id,
	})

	emote := &datastructure.Emote{}

	err = res.Err()

	if err == nil {
		err = res.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownEmote
		}
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	if !usr.HasPermission(datastructure.RolePermissionEmoteEditAll) {
		if emote.OwnerID.Hex() != usr.ID.Hex() {
			if err := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, bson.M{
				"_id":     emote.OwnerID,
				"editors": usr.ID,
			}).Err(); err != nil {
				if err == mongo.ErrNoDocuments {
					return nil, resolvers.ErrAccessDenied
				}
				log.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}
		}
	}

	if req.Name != nil {
		if emote.Name != update["name"] {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "name",
				OldValue: emote.Name,
				NewValue: update["name"],
			})
		}
	}
	if req.OwnerID != nil {
		if emote.OwnerID != update["owner"] {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "owner",
				OldValue: emote.OwnerID,
				NewValue: update["owner"],
			})
		}
	}
	if req.Tags != nil {
		if utils.DifferentArray(emote.Tags, update["tags"].([]string)) {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "tags",
				OldValue: emote.Tags,
				NewValue: update["tags"],
			})
		}
	}
	if req.Visibility != nil {
		if !usr.HasPermission(datastructure.RolePermissionEmoteEditAll) {
			// User tries to make the emote global but lacks permission
			if utils.BitField.HasBits(int64(*req.Visibility), int64(datastructure.EmoteVisibilityGlobal)) && !usr.HasPermission(datastructure.RolePermissionEditEmoteGlobalState) {
				return nil, resolvers.ErrAccessDenied // User tries to set emote's global state but lacks permission
			}

			// User tries to remove the unlisted state but lacks permission
			if utils.BitField.HasBits(int64(emote.Visibility), int64(datastructure.EmoteVisibilityUnlisted)) && !utils.BitField.HasBits(int64(*req.Visibility), int64(datastructure.EmoteVisibilityUnlisted)) {
				return nil, resolvers.ErrAccessDenied
			}
			if utils.BitField.HasBits(int64(emote.Visibility), int64(datastructure.EmoteVisibilityPermanentlyUnlisted)) && !utils.BitField.HasBits(int64(*req.Visibility), int64(datastructure.EmoteVisibilityPermanentlyUnlisted)) {
				return nil, resolvers.ErrAccessDenied
			}
		}

		if emote.Visibility != update["visibility"] {
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "visibility",
				OldValue: emote.Visibility,
				NewValue: update["visibility"],
			})
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	if len(logChanges) > 0 {
		update["last_modified_date"] = time.Now()

		oldVisibility := emote.Visibility
		after := options.After
		doc := mongo.Collection(mongo.CollectionNameEmotes).FindOneAndUpdate(ctx, bson.M{
			"_id": id,
		}, bson.M{
			"$set": update,
		}, &options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
		})
		if err := doc.Decode(emote); err != nil {
			return nil, err
		}

		err = doc.Err()
		if err != nil {
			log.WithError(err).WithField("id", id).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}

		_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
			Type:      datastructure.AuditLogTypeEmoteEdit,
			CreatedBy: usr.ID,
			Target:    &datastructure.Target{ID: &id, Type: "emotes"},
			Changes:   logChanges,
			Reason:    args.Reason,
		})

		if err != nil {
			log.WithError(err).Error("mongo")
		}

		// Send a notification to the emote owner if another user removed the UNLISTED flag
		if usr.ID.Hex() != emote.OwnerID.Hex() {
			wasUnlisted := utils.BitField.HasBits(int64(oldVisibility), int64(datastructure.EmoteVisibilityUnlisted)) &&
				!utils.BitField.HasBits(int64(emote.Visibility), int64(datastructure.EmoteVisibilityUnlisted))

			if req.Visibility != nil && wasUnlisted {
				notification := actions.Notifications.Create().
					SetTitle("Emote Approved").
					AddTargetUsers(emote.OwnerID).
					AddTextMessagePart("Your emote ").
					AddEmoteMentionPart(emote.ID).
					AddTextMessagePart("was approved by ").
					AddUserMentionPart(usr.ID).
					AddTextMessagePart("!")

				go func() {
					// Send the notification
					if err := notification.Write(context.Background()); err != nil {
						log.WithError(err).Error("failed to create notification")
					}
				}()
			}

		}

		go discord.SendEmoteEdit(*emote, *usr, logChanges, args.Reason)
		return query_resolvers.GenerateEmoteResolver(ctx, emote, &emote.ID, field.Children)
	}

	return query_resolvers.GenerateEmoteResolver(ctx, emote, &emote.ID, field.Children)
}
