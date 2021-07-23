package mutation_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// ADD CHANNEL EDITOR
//
func (*MutationResolver) AddChannelEditor(ctx context.Context, args struct {
	ChannelID string
	EditorID  string
	Reason    *string
}) (*query_resolvers.UserResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	editorID, err := primitive.ObjectIDFromHex(args.EditorID)
	if err != nil {
		return nil, resolvers.ErrUnknownUser
	}

	channelID, err := primitive.ObjectIDFromHex(args.ChannelID)
	if err != nil {
		return nil, resolvers.ErrUnknownChannel
	}

	// Can't add self as editor...
	if editorID.Hex() == channelID.Hex() {
		return nil, resolvers.ErrYourself
	}

	_, err = redis.Client.HGet(ctx, "user:bans", channelID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	_, err = redis.Client.HGet(ctx, "user:bans", editorID.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, bson.M{
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
			return nil, resolvers.ErrAccessDenied
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	var newChannel *datastructure.User
	after := options.After
	doc := mongo.Collection(mongo.CollectionNameUsers).FindOneAndUpdate(ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$addToSet": bson.M{
			"editors": editorID,
		},
	}, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(&newChannel); err != nil {
		return nil, err
	}

	if err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserChannelEditorAdd,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &channelID, Type: "users"},
		Changes: []*datastructure.AuditLogChange{
			{Key: "editors", OldValue: nil, NewValue: editorID},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}
	return query_resolvers.GenerateUserResolver(ctx, newChannel, &newChannel.ID, field.Children)
}

//
// REMOVE CHANNEL EDITOR
//
func (*MutationResolver) RemoveChannelEditor(ctx context.Context, args struct {
	ChannelID string
	EditorID  string
	Reason    *string
}) (*query_resolvers.UserResolver, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	editorID, err := primitive.ObjectIDFromHex(args.EditorID)
	if err != nil {
		return nil, resolvers.ErrUnknownUser
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

	res := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, bson.M{
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
			return nil, resolvers.ErrAccessDenied
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	var newChannel *datastructure.User
	after := options.After
	doc := mongo.Collection(mongo.CollectionNameUsers).FindOneAndUpdate(ctx, bson.M{
		"_id": channelID,
	}, bson.M{
		"$pull": bson.M{
			"editors": editorID,
		},
	}, &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	})
	if err := doc.Decode(&newChannel); err != nil {
		return nil, err
	}

	if err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserChannelEditorRemove,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &channelID, Type: "users"},
		Changes: []*datastructure.AuditLogChange{
			{Key: "editors", OldValue: nil, NewValue: editorID},
		},
		Reason: args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	return query_resolvers.GenerateUserResolver(ctx, newChannel, &newChannel.ID, field.Children)
}
