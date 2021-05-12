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
)

//
// BAN USER
//
func (*MutationResolver) BanUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	if !datastructure.UserHasPermission(usr, datastructure.RolePermissionBanUsers) {
		return nil, resolvers.ErrAccessDenied
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, resolvers.ErrUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, resolvers.ErrYourself
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil && err != redis.ErrNil {
		log.Errorf("redis, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	if err == nil {
		return nil, resolvers.ErrUserBanned
	}

	res := mongo.Database.Collection("user").FindOne(mongo.Ctx, bson.M{
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
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	if user.Role.Position >= usr.Role.Position {
		return nil, resolvers.ErrAccessDenied
	}

	reasonN := "Not Provided"
	if args.Reason != nil {
		reasonN = *args.Reason
	}

	ban := &datastructure.Ban{
		UserID:     &user.ID,
		Active:     true,
		Reason:     reasonN,
		IssuedByID: &usr.ID,
	}

	_, err = mongo.Database.Collection("bans").InsertOne(mongo.Ctx, ban)
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	_, err = redis.Client.HSet(redis.Ctx, "user:bans", id.Hex(), reasonN).Result()
	if err != nil {
		log.Errorf("redis, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserBan,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "users"},
		Changes:   nil,
		Reason:    args.Reason,
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}

//
// UNBAN USER
//

func (*MutationResolver) UnbanUser(ctx context.Context, args struct {
	UserID string
	Reason *string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	if !datastructure.UserHasPermission(usr, datastructure.RolePermissionBanUsers) {
		return nil, resolvers.ErrAccessDenied
	}

	id, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, resolvers.ErrUnknownUser
	}

	if id.Hex() == usr.ID.Hex() {
		return nil, resolvers.ErrYourself
	}

	_, err = redis.Client.HGet(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil {
		if err != redis.ErrNil {
			return nil, resolvers.ErrUserNotBanned
		}
		log.Errorf("redis, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	res := mongo.Database.Collection("user").FindOne(mongo.Ctx, bson.M{
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
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	if user.Role.Position >= usr.Role.Position {
		return nil, resolvers.ErrAccessDenied
	}

	_, err = mongo.Database.Collection("bans").UpdateMany(mongo.Ctx, bson.M{
		"user_id": user.ID,
		"active":  true,
	}, bson.M{
		"$set": bson.M{
			"active": false,
		},
	})
	if err != nil {
		log.Errorf("mongo, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	_, err = redis.Client.HDel(redis.Ctx, "user:bans", id.Hex()).Result()
	if err != nil {
		log.Errorf("redis, err=%v", err)
		return nil, resolvers.ErrInternalServer
	}

	_, err = mongo.Database.Collection("audit").InsertOne(mongo.Ctx, &datastructure.AuditLog{
		Type:      datastructure.AuditLogTypeUserUnban,
		CreatedBy: usr.ID,
		Target:    &datastructure.Target{ID: &id, Type: "users"},
		Changes:   nil,
		Reason:    args.Reason,
	})

	if err != nil {
		log.Errorf("mongo, err=%v", err)
	}

	return &response{
		Status:  200,
		Message: "success",
	}, nil
}