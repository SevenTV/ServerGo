package actions

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Write: Save this Entitlement to persistence
func (b EntitlementBuilder) Write() error {
	// Create new Object ID if this is a new entitlement
	if b.Entitlement.ID.IsZero() {
		b.Entitlement.ID = primitive.NewObjectID()
	}

	if _, err := mongo.Database.Collection("entitlements").UpdateByID(b.ctx, b.Entitlement.ID, bson.M{
		"$set": b.Entitlement,
	}, &options.UpdateOptions{
		Upsert: utils.BoolPointer(true),
	}); err != nil {
		log.WithError(err).Error("mongo")
		return err
	}

	return nil
}

// GetUser: Fetch the user data from the user ID assigned to the entitlement
func (b EntitlementBuilder) GetUser() (*UserBuilder, error) {
	if b.Entitlement.UserID.IsZero() {
		return nil, fmt.Errorf("Entitlement does not have a user assigned")
	}

	ub, err := Users.GetByID(b.ctx, b.Entitlement.UserID)
	if err != nil {
		return nil, err
	}

	role := datastructure.GetRole(ub.User.RoleID)
	ub.User.Role = &role
	return ub, nil
}

// SetKind: Change the entitlement's kind
func (b EntitlementBuilder) SetKind(kind datastructure.EntitlementKind) EntitlementBuilder {
	b.Entitlement.Kind = kind

	return b
}

// SetUserID: Change the entitlement's assigned user
func (b EntitlementBuilder) SetUserID(id primitive.ObjectID) EntitlementBuilder {
	b.Entitlement.UserID = id

	return b
}

// SetSubscriptionData: Add a subscription reference to the entitlement
func (b EntitlementBuilder) SetSubscriptionData(data datastructure.EntitledSubscription) EntitlementBuilder {
	return b.marshalData(data)
}

// SetBadgeData: Add a badge reference to the entitlement
func (b EntitlementBuilder) SetBadgeData(data datastructure.EntitledBadge) EntitlementBuilder {
	return b.marshalData(data)
}

// SetRoleData: Add a role reference to the entitlement
func (b EntitlementBuilder) SetRoleData(data datastructure.EntitledRole) EntitlementBuilder {
	return b.marshalData(data)
}

// SetEmoteSetData: Add an emote set reference to the entitlement
func (b EntitlementBuilder) SetEmoteSetData(data datastructure.EntitledEmoteSet) EntitlementBuilder {
	return b.marshalData(data)
}

func (b EntitlementBuilder) marshalData(data interface{}) EntitlementBuilder {
	d, err := bson.Marshal(data)
	if err != nil {
		log.WithError(err).Error("bson")
		return b
	}

	b.Entitlement.Data = d
	return b
}

// ReadSubscriptionData: Read the data as an Entitled Subscription
func (b EntitlementBuilder) ReadSubscriptionData() datastructure.EntitledSubscription {
	var e datastructure.EntitledSubscription
	if err := bson.Unmarshal(b.Entitlement.Data, &e); err != nil {
		log.WithError(err).Error("bson")
		return e
	}
	return e
}

// ReadBadgeData: Read the data as an Entitled Badge
func (b EntitlementBuilder) ReadBadgeData() datastructure.EntitledBadge {
	var e datastructure.EntitledBadge
	if err := bson.Unmarshal(b.Entitlement.Data, &e); err != nil {
		log.WithError(err).Error("bson")
		return e
	}
	return e
}

// ReadRoleData: Read the data as an Entitled Role
func (b EntitlementBuilder) ReadRoleData() datastructure.EntitledRole {
	var e datastructure.EntitledRole
	if err := bson.Unmarshal(b.Entitlement.Data, &e); err != nil {
		log.WithError(err).Error("bson")
		return e
	}
	return e
}

// ReadEmoteSetData: Read the data as an Entitled Emote Set
func (b EntitlementBuilder) ReadEmoteSetData() datastructure.EntitledEmoteSet {
	var e datastructure.EntitledEmoteSet
	if err := bson.Unmarshal(b.Entitlement.Data, &e); err != nil {
		log.WithError(err).Error("bson")
		return e
	}
	return e
}

// Create: Get a new entitlement builder
func (entitlements) Create(ctx context.Context) EntitlementBuilder {
	return EntitlementBuilder{
		Entitlement: datastructure.Entitlement{},
	}
}

// With: Get an entitledment builder tied to an entitlement
func (entitlements) With(ctx context.Context, e datastructure.Entitlement) EntitlementBuilder {
	return EntitlementBuilder{
		Entitlement: e,
	}
}

// --- Syncing Methods ---

// Sync: Apply the Entitlement
// For example, grant the role to the user for a Role entitlement
func (b EntitlementBuilder) Sync() error {
	x := map[datastructure.EntitlementKind]func() error{
		datastructure.EntitlementKindSubscription: b.syncSubscription,
		datastructure.EntitlementKindBadge:        b.syncBadge,
		datastructure.EntitlementKindRole:         b.syncRole,
		datastructure.EntitlementKindEmoteSet:     b.syncEmoteSet,
	}

	f, ok := x[b.Entitlement.Kind]
	if !ok {
		return fmt.Errorf("Cannot sync with kind %v", b.Entitlement.Kind)
	}

	if err := f(); err != nil {
		return err
	} else {
		b.Log(fmt.Sprintf("Synced successfully"))
	}

	return nil
}

func (b EntitlementBuilder) syncSubscription() error {
	return nil
}
func (b EntitlementBuilder) syncBadge() error {
	return nil
}
func (b EntitlementBuilder) syncRole() error {
	return nil
}
func (b EntitlementBuilder) syncEmoteSet() error {
	return nil
}

func (b EntitlementBuilder) Log(str string) {
	log.Infof("<EntitlementBuilder:%v> [Kind: %v] [User: %v] %v", b.Entitlement.ID, b.Entitlement.Kind, b.Entitlement.UserID, str)
}

func (b EntitlementBuilder) LogError(str string) {
	log.Errorf("<EntitlementBuilder:%v> [Kind: %v] [User: %v] %v", b.Entitlement.ID, b.Entitlement.Kind, b.Entitlement.UserID, str)
}
