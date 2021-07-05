package actions

import (
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
