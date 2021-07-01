package actions

import (
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type notifications struct{}

type NotificationBuilder struct {
	Notification datastructure.Notification
}

// Set the Notification's Title
func (b NotificationBuilder) SetTitle(title string) NotificationBuilder {
	b.Notification.Content.Title = title

	return b
}

// Append a Text part to the notification
func (b NotificationBuilder) AddTextMessagePart(text string) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type: datastructure.NotificationContentMessagePartTypeText,
		Text: &text,
	})

	return b
}

// Add one or more users who may read this notification
func (b NotificationBuilder) AddTargetUsers(userIDs ...primitive.ObjectID) NotificationBuilder {
	b.Notification.TargetUsers = append(b.Notification.TargetUsers, userIDs...)

	return b
}

// Add one or more roles that may allow their members to read this notification
func (b NotificationBuilder) AddTargetRoles(roleIDs ...primitive.ObjectID) NotificationBuilder {
	b.Notification.TargetRoles = append(b.Notification.TargetRoles, roleIDs...)

	return b
}

// Mark this notification as an announcement, meaning all users will be able to read it
// regardless of the selected targets
func (b NotificationBuilder) MarkAsAnnouncement() NotificationBuilder {
	b.Notification.Announcement = true

	return b
}

// The users who've read this notification
func (b NotificationBuilder) SetReadBy(userIDs ...primitive.ObjectID) NotificationBuilder {
	b.Notification.ReadBy = append(b.Notification.ReadBy, userIDs...)

	return b
}

// Get a NotificationBuilder
func (*notifications) Create() NotificationBuilder {
	builder := NotificationBuilder{
		Notification: datastructure.Notification{},
	}

	return builder
}

// Get a NotificationBuilder populated with an existing notification
func (*notifications) CreateFrom(notification datastructure.Notification) NotificationBuilder {
	builder := NotificationBuilder{
		Notification: notification,
	}

	return builder
}
