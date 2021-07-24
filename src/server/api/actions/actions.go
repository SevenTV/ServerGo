package actions

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type emotes struct{}

var Emotes emotes = emotes{}

type notifications struct{}

type NotificationBuilder struct {
	Notification    datastructure.Notification
	MentionedUsers  []primitive.ObjectID
	MentionedEmotes []primitive.ObjectID
	MentionedRoles  []primitive.ObjectID
	TargetUsers     []primitive.ObjectID
}

var Notifications notifications = notifications{}

type entitlements struct{}

type EntitlementBuilder struct {
	Entitlement datastructure.Entitlement
	ctx         context.Context

	User *datastructure.User
}

var Entitlements = entitlements{}

type users struct{}

type UserBuilder struct {
	User datastructure.User
}

var Users users = users{}
