package mutation_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (*MutationResolver) EditUser(ctx context.Context, args struct {
	User   userInput
	Reason *string
}) (*query_resolvers.UserResolver, error) {
	// Get the actor user
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Get the target user
	targetID, err := primitive.ObjectIDFromHex(args.User.ID)
	if err != nil {
		return nil, err
	}
	var target *datastructure.User
	if err := mongo.Database.Collection("users").FindOne(ctx, bson.M{"_id": targetID}).Decode(&target); err != nil {
		return nil, err
	}
	// Get target's role
	targetRole := datastructure.GetRole(target.RoleID)
	target.Role = &targetRole

	update := bson.M{}
	logChanges := []*datastructure.AuditLogChange{}
	req := args.User

	// Update: Role
	if req.RoleID != nil {
		// Check actor can manage roles
		if !usr.HasPermission(datastructure.RolePermissionManageRoles) {
			return nil, resolvers.ErrAccessDenied
		}

		// Check actor user's permission to edit target's role
		if usr.Role.Position <= target.Role.Position {
			return nil, resolvers.ErrAccessDenied
		}

		// Role is empty?
		if *req.RoleID == "" {
			update["role"] = nil
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "role",
				OldValue: target.RoleID,
				NewValue: nil,
			})
		} else {
			// Find role with ID
			roleID, err := primitive.ObjectIDFromHex(*req.RoleID)
			if err != nil {
				return nil, err
			}
			role := datastructure.GetRole(&roleID)
			if role.Default {
				return nil, resolvers.ErrUnknownRole
			}

			// Update role
			update["role"] = role.ID
			logChanges = append(logChanges, &datastructure.AuditLogChange{
				Key:      "role",
				OldValue: target.RoleID,
				NewValue: role.ID,
			})
		}
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	var user *datastructure.User
	if len(logChanges) > 0 {
		after := options.After
		doc := mongo.Database.Collection("users").FindOneAndUpdate(ctx, bson.M{
			"_id": targetID,
		}, bson.M{
			"$set": update,
		}, &options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
		})
		if err := doc.Decode(&user); err != nil {
			return nil, resolvers.ErrInternalServer
		}

		_, err := mongo.Database.Collection("audit").InsertOne(ctx, &datastructure.AuditLog{
			Type:      datastructure.AuditLogTypeUserEdit,
			CreatedBy: usr.ID,
			Target:    &datastructure.Target{ID: &targetID, Type: "users"},
			Changes:   logChanges,
			Reason:    args.Reason,
		})
		if err != nil {
			log.WithError(err).Error("mongo")
		}

		return query_resolvers.GenerateUserResolver(ctx, user, &targetID, field.Children)
	}

	return query_resolvers.GenerateUserResolver(ctx, target, &targetID, field.Children)
}
