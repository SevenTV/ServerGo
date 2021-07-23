package mutation_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//
// REPORT EMOTE
//
func (*MutationResolver) ReportEmote(ctx context.Context, args struct {
	EmoteID string
	Reason  *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.EmoteID)
	if err != nil {
		return nil, resolvers.ErrUnknownEmote
	}

	res := mongo.Collection(mongo.CollectionNameEmotes).FindOne(ctx, bson.M{
		"_id":    id,
		"status": datastructure.EmoteStatusLive,
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

	opts := options.Update().SetUpsert(true)

	_, err = mongo.Collection(mongo.CollectionNameReports).UpdateOne(ctx, bson.M{
		"target.id":   emote.ID,
		"target.type": "emotes",
		"cleared":     false,
		"reporter_id": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"target.id":   emote.ID,
			"target.type": "emotes",
			"cleared":     false,
			"reporter_id": usr.ID,
			"reason":      args.Reason,
		},
	}, opts)

	if err != nil {
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeReport,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "emotes"},
		Changes:   nil,
		Reason:    args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

//
// REPORT USER
//
func (*MutationResolver) ReportUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, resolvers.ErrUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, resolvers.ErrYourself
	}

	_, err = redis.Client.HGet(ctx, "user:bans", id.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Database.Collection("user").FindOne(ctx, bson.M{
		"_id": id,
	})

	user := &datastructure.User{}

	err = res.Err()

	if err == nil {
		err = res.Decode(user)
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, resolvers.ErrUnknownUser
		}
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	opts := options.Update().SetUpsert(true)

	_, err = mongo.Collection(mongo.CollectionNameReports).UpdateOne(ctx, bson.M{
		"target.id":   user.ID,
		"target.type": "users",
		"cleared":     false,
		"reporter_id": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"target.id":   user.ID,
			"target.type": "users",
			"cleared":     false,
			"reporter_id": usr.ID,
			"reason":      args.Reason,
		},
	}, opts)

	if err != nil {
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeReport,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "emotes"},
		Changes:   nil,
		Reason:    args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("mongo")
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}
