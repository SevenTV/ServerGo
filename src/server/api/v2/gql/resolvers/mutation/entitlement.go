package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
	}

	// Write to DB
	if err := builder.Write(); err != nil {
		log.WithError(err).Error(err)
		return nil, resolvers.ErrInternalServer
	}

	// Sync the entitlement
	if err := builder.Sync(); err != nil {
		builder.LogError(fmt.Sprintf("Couldn't sync entitlement: %v", err.Error()))
	}

	return &response{
		OK:      true,
		Message: "Entitlement Created",
	}, nil
}
