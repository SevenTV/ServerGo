package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (*MutationResolver) DeleteEntitlement(ctx context.Context, args struct {
	ID string
}) (*response, error) {
	// Get actor reference
	actor, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Verify actor's permission
	if !actor.HasPermission(datastructure.RolePermissionManageEntitlements) {
		return nil, resolvers.ErrAccessDenied
	}

	// Parse ID to objectid
	eID, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, err
	}

	// Delete the entitlement
	if _, err = mongo.Collection(mongo.CollectionNameEntitlements).DeleteOne(ctx, bson.M{
		"_id": eID,
	}); err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	return &response{
		OK:      true,
		Message: "Entitlement Deleted",
	}, nil
}

func (*MutationResolver) CreateEntitlement(ctx context.Context, args struct {
	Kind     datastructure.EntitlementKind
	Data     entitlementCreateInput
	UserID   string
	Disabled *bool
}) (*response, error) {
	// Get actor reference
	actor, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Verify actor's permission
	if !actor.HasPermission(datastructure.RolePermissionManageEntitlements) {
		return nil, resolvers.ErrAccessDenied
	}

	// Parse entitled user ID
	userID, err := primitive.ObjectIDFromHex(args.UserID)
	if err != nil {
		return nil, err
	}

	// Create an entitlement builder and assign kind+user ID
	builder := actions.Entitlements.Create(ctx).
		SetKind(args.Kind).
		SetUserID(userID)

	// Initiate a new notification to be sent to the entitled user
	notify := actions.Notifications.Create().
		AddTargetUsers(userID)

	// Assign typed data based on kind
	var itemID primitive.ObjectID
	switch args.Kind {
	case datastructure.EntitlementKindSubscription:
		if args.Data.Subscription == nil {
			return nil, fmt.Errorf("Missing Subscription Data")
		}
		itemID, err = primitive.ObjectIDFromHex(args.Data.Subscription.ID)
		if err != nil {
			return nil, err
		}

		// Set Subscription Data to builder
		builder = builder.SetSubscriptionData(datastructure.EntitledSubscription{
			ObjectReference: itemID,
		})
	case datastructure.EntitlementKindBadge:
		if args.Data.Badge == nil {
			return nil, fmt.Errorf("Missing Badge Data")
		}
		itemID, err = primitive.ObjectIDFromHex(args.Data.Badge.ID)
		if err != nil {
			return nil, err
		}

		builder = builder.SetBadgeData(datastructure.EntitledBadge{
			ObjectReference: itemID,
			Selected:        args.Data.Badge.Selected,
		})
	case datastructure.EntitlementKindRole:
		if args.Data.Role == nil {
			return nil, fmt.Errorf("Missing Role Data")
		}
		itemID, err = primitive.ObjectIDFromHex(args.Data.Role.ID)
		if err != nil {
			return nil, err
		}

		// Set Role Data to builder
		builder = builder.SetRoleData(datastructure.EntitledRole{
			ObjectReference: itemID,
		})

		role := datastructure.GetRole(&itemID)
		notify = notify.SetTitle("Global Role Granted").
			AddTextMessagePart("You've been granted the role").
			AddRoleMentionPart(role.ID).
			AddTextMessagePart("by").
			AddUserMentionPart(actor.ID).
			AddTextMessagePart(".")
	}

	// Write to DB
	if builder, err = builder.Write(); err != nil {
		log.WithError(err).Error(err)
		return nil, resolvers.ErrInternalServer
	}

	// Sync the entitlement
	if err := builder.Sync(); err != nil {
		builder.LogError(fmt.Sprintf("Couldn't sync entitlement: %v", err.Error()))
	}

	// Add the X-Created-ID header specifying the ID of the entitlement created
	f, ok := ctx.Value(utils.RequestCtxKey).(*fiber.Ctx) // Fiber context
	if ok {
		f.Set("X-Created-ID", builder.Entitlement.ID.Hex())
	}

	// Send the notification
	if notify.Notification.Title != "" {
		go func() {
			if err := notify.Write(ctx); err != nil {
				log.WithError(err).Error("notifications")
			}
		}()
	}

	return &response{
		OK:      true,
		Message: "Entitlement Created",
	}, nil
}
