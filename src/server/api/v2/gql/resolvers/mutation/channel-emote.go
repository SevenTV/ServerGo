package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/SevenTV/ServerGo/src/validation"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Mutate Emote - Add to Channel
func (*MutationResolver) AddChannelEmote(ctx context.Context, args struct {
	ChannelID string
	EmoteID   string
	Reason    *string
}) (*query_resolvers.UserResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	emoteID, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	channelID, err := primitive.ObjectIDFromHex(args.ChannelID)
	if err != nil {
		return nil, resolvers.ErrUnknownChannel
	}

	_, err = redis.Client.HGet(ctx, "user:bans", channelID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Database.Collection("users").FindOne(ctx, bson.M{
		"_id": channelID,
	})

	channel := &datastructure.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(channel)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownChannel
		}
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
		if channel.ID.Hex() != usr.ID.Hex() {
			found := false
			for _, e := range channel.EditorIDs {
				if e.Hex() == usr.ID.Hex() {
					found = true
					break
				}
			}
			if !found {
				return nil, resolvers.ErrAccessDenied
			}
		}

		if (len(channel.EmoteIDs) + 1) > int(channel.GetEmoteSlots()) {
			return nil, resolvers.ErrEmoteSlotLimitReached(channel.GetEmoteSlots())
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	for _, eID := range channel.EmoteIDs {
		if eID.Hex() == emoteID.Hex() {
			return query_resolvers.GenerateUserResolver(ctx, channel, &channelID, field.Children)
		}
	}

	emoteRes := mongo.Database.Collection("emotes").FindOne(ctx, bson.M{
		"_id":    emoteID,
		"status": datastructure.EmoteStatusLive,
	})

	emote := &datastructure.Emote{}
	err = emoteRes.Err()
	if err == nil {
		err = emoteRes.Decode(emote)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownEmote
		}
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	sharedWith := []string{}
	for _, v := range emote.SharedWith {
		sharedWith = append(sharedWith, v.Hex())
	}

	if utils.BitField.HasBits(int64(emote.Visibility), int64(datastructure.EmoteVisibilityPrivate)) {
		if emote.OwnerID.Hex() != channelID.Hex() && !utils.Contains(sharedWith, emoteID.Hex()) {
			return nil, resolvers.ErrUnknownEmote
		}
	}

	emoteIDs := append(channel.EmoteIDs, emoteID)
	after := options.After
	doc := mongo.Database.Collection("users").FindOneAndUpdate(ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"emotes": emoteIDs,
		},
	}, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(channel); err != nil {
		return nil, err
	}

	if doc.Err() != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserChannelEmoteAdd,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &channelID, Type: "users"},
		Changes: []*datastructure.AuditLogChange{
			{Key: "emotes", OldValue: nil, NewValue: emoteID},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	// Push event to redis
	{
		ids := make([]string, len(channel.EmoteIDs))
		for i, id := range channel.EmoteIDs {
			ids[i] = id.Hex()
		}

		_ = redis.Publish(ctx, fmt.Sprintf("users:%v:emotes", channel.Login), redis.PubSubPayloadUserEmotes{
			Removed: false,
			ID:      emoteID.Hex(),
			Actor:   usr.DisplayName,
		})
	}
	return query_resolvers.GenerateUserResolver(ctx, channel, &channelID, field.Children)
}

// Mutate Emote
func (*MutationResolver) EditChannelEmote(ctx context.Context, args struct {
	ChannelID string
	EmoteID   string
	Data      struct {
		Alias *string
	}
	Reason *string
}) (*query_resolvers.UserResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	emoteID, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	channelID, err := primitive.ObjectIDFromHex(args.ChannelID)
	if err != nil {
		return nil, resolvers.ErrUnknownChannel
	}

	_, err = redis.Client.HGet(ctx, "user:bans", channelID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Database.Collection("users").FindOne(ctx, bson.M{
		"_id": channelID,
	})
	channel := &datastructure.User{}
	err = res.Err()

	if err == nil {
		err = res.Decode(channel)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownChannel
		}
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	// Check permissions
	if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
		if channel.ID.Hex() != usr.ID.Hex() {
			found := false
			for _, e := range channel.EditorIDs {
				if e.Hex() == usr.ID.Hex() {
					found = true
					break
				}
			}
			if !found {
				return nil, resolvers.ErrAccessDenied
			}
		}
	}

	update := bson.M{}
	set := bson.M{}
	unset := bson.M{}
	logChanges := []*datastructure.AuditLogChange{}
	if args.Data.Alias != nil {
		alias := *args.Data.Alias
		if alias == "" {
			unset = bson.M{
				fmt.Sprintf("emote_alias.%v", emoteID.Hex()): "",
			}
		} else {
			if valid := validation.ValidateEmoteName(utils.S2B(alias)); !valid {
				return nil, resolvers.ErrInvalidName
			}

			set[fmt.Sprintf("emote_alias.%v", emoteID.Hex())] = alias
		}

		logChanges = append(logChanges, &datastructure.AuditLogChange{
			Key: "emote_alias", OldValue: channel.EmoteAlias[emoteID.Hex()], NewValue: alias,
		})
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	// Format update
	if len(set) > 0 {
		update["$set"] = set
	}
	if len(unset) > 0 {
		update["$unset"] = unset
	}

	fmt.Println("alias:", update, args.Data.Alias)
	after := options.After
	doc := mongo.Database.Collection("users").FindOneAndUpdate(ctx, bson.M{
		"_id": channelID,
	}, update, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(channel); err != nil {
		return nil, err
	}

	if doc.Err() != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserChannelEmoteEdit,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &channelID, Type: "users"},
		Changes:   logChanges,
		Reason:    args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	// Push event to redis
	{
		ids := make([]string, len(channel.EmoteIDs))
		for i, id := range channel.EmoteIDs {
			ids[i] = id.Hex()
		}

		_ = redis.Publish(ctx, fmt.Sprintf("users:%v:emotes", channel.Login), redis.PubSubPayloadUserEmotes{
			Removed: false,
			ID:      emoteID.Hex(),
			Actor:   usr.DisplayName,
		})
	}
	return query_resolvers.GenerateUserResolver(ctx, channel, &channelID, field.Children)
}

// Mutate Emote - Remove from Channel
func (*MutationResolver) RemoveChannelEmote(ctx context.Context, args struct {
	ChannelID string
	EmoteID   string
	Reason    *string
}) (*query_resolvers.UserResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	emoteID, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	channelID, err := primitive.ObjectIDFromHex(args.ChannelID)
	if err != nil {
		return nil, resolvers.ErrUnknownChannel
	}

	_, err = redis.Client.HGet(ctx, "user:bans", channelID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Database.Collection("users").FindOne(ctx, bson.M{
		"_id": channelID,
	})

	channel := &datastructure.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(channel)
	}
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownChannel
		}
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
		if channel.ID.Hex() != usr.ID.Hex() {
			found := false
			for _, e := range channel.EditorIDs {
				if e.Hex() == usr.ID.Hex() {
					found = true
					break
				}
			}
			if !found {
				return nil, resolvers.ErrAccessDenied
			}
		}
	}

	found := false

	newIds := []primitive.ObjectID{}

	for _, eID := range channel.EmoteIDs {
		if eID.Hex() == emoteID.Hex() {
			found = true
		} else {
			newIds = append(newIds, eID)
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	if !found {
		return query_resolvers.GenerateUserResolver(ctx, channel, &channelID, field.Children)
	}

	after := options.After
	doc := mongo.Database.Collection("users").FindOneAndUpdate(ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$set": bson.M{
			"emotes": newIds,
		},
	}, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(channel); err != nil {
		return nil, err
	}

	if doc.Err() != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserChannelEmoteRemove,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &channelID, Type: "users"},
		Changes: []*datastructure.AuditLogChange{
			{Key: "emotes", OldValue: nil, NewValue: emoteID},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	// Push event to redis
	{
		ids := make([]string, len(channel.EmoteIDs))
		for i, id := range channel.EmoteIDs {
			ids[i] = id.Hex()
		}

		_ = redis.Publish(ctx, fmt.Sprintf("users:%v:emotes", channel.Login), redis.PubSubPayloadUserEmotes{
			Removed: true,
			ID:      emoteID.Hex(),
			Actor:   usr.DisplayName,
		})
	}
	return query_resolvers.GenerateUserResolver(ctx, channel, &channelID, field.Children)
}
