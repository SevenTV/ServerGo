package actions

import (
	"context"
	"fmt"
	"sort"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (users) With(ctx context.Context, user *datastructure.User) (UserBuilder, error) {
	builder := UserBuilder{}
	if user == nil {
		return builder, fmt.Errorf("User passed is nil")
	}

	builder.User = *user
	builder.ctx = ctx
	return builder, nil
}

// GetByID: fetch a user via their ID
func (x users) GetByID(ctx context.Context, id primitive.ObjectID) (*UserBuilder, error) {
	b, err := x.Get(ctx, bson.M{"_id": id})
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Get: fetch a user via a query
func (users) Get(ctx context.Context, q bson.M) (*UserBuilder, error) {
	res := mongo.Collection(mongo.CollectionNameUsers).FindOne(ctx, q)
	err := res.Err()

	if err != nil {
		return nil, err
	}

	var user datastructure.User
	if err := res.Decode(&user); err != nil {
		return nil, err
	}

	role := datastructure.GetRole(user.RoleID)
	user.Role = &role
	builder := UserBuilder{
		user,
		ctx,
		[]EntitlementBuilder{},
	}

	return &builder, nil
}

// GetRole: Returns the user's current role
func (b UserBuilder) GetRole() datastructure.Role {
	ents, _ := b.FetchEntitlements(&datastructure.EntitlementKindRole)
	for i, ent := range ents {
		ents[i] = ent
	}

	// Sort the entitlements by role position
	sort.Slice(ents, func(i, j int) bool {
		roleID := ents[i].ReadRoleData().ObjectReference
		role1 := datastructure.GetRole(&roleID)
		role2 := datastructure.GetRole(b.User.RoleID)

		return role1.Position > role2.Position
	})

	// The role assigned to the user directly via their user rather than an Entitlement
	hardRole := datastructure.GetRole(b.User.RoleID)

	if len(ents) > 0 {
		refID := ents[0].ReadRoleData().ObjectReference
		refRole := datastructure.GetRole(&refID)

		// If the hard role has the same or higher position, it will be returned instead of the entitled role
		return utils.Ternary(hardRole.Position >= refRole.Position, hardRole, refRole).(datastructure.Role)
	} else {
		return hardRole
	}
}

// AssignEntitlements: adds entitlements to the user object
func (b UserBuilder) FetchEntitlements(kind *datastructure.EntitlementKind) ([]EntitlementBuilder, error) {
	// Make a request to get the user's entitlements
	var entitlements []*datastructure.Entitlement
	cur, err := mongo.Collection(mongo.CollectionNameEntitlements).Find(b.ctx, bson.M{
		"user_id":  b.User.ID,
		"kind":     kind,
		"disabled": bson.M{"$not": bson.M{"$eq": true}},
	})
	if err == mongo.ErrNoDocuments {
		return nil, nil
	} else if err != nil {
		log.WithError(err).Error("actions, UserBuilder, FetchEntitlements")
		return nil, err
	}

	// Get all entitlements
	if err := cur.All(b.ctx, &entitlements); err != nil {
		return nil, err
	}

	// Wrap into Entitlement Builders
	builders := make([]EntitlementBuilder, len(entitlements))
	for i, e := range entitlements {
		builders[i] = EntitlementBuilder{
			Entitlement: *e,
			ctx:         b.ctx,
			User:        &b.User,
		}
	}

	b.Entitlements = builders // Assign entitlements to UserBuilder
	return builders, nil
}
