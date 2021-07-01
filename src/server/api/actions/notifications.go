package actions

import (
	"context"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type notifications struct{}

type NotificationBuilder struct {
	Notification datastructure.Notification
}

// Get the data of mentioned users in the notification's message parts
func (b NotificationBuilder) GetMentionedUsers(ctx context.Context) (NotificationBuilder, error) {
	var userIDs []primitive.ObjectID
	for _, part := range b.Notification.Content.MessageParts { // Check message parts for user mentions
		if part.Type != datastructure.NotificationContentMessagePartTypeUserMention {
			continue
		}
		if part.Mention == nil {
			continue
		}

		// Append unique user IDs to slice
		mention := *part.Mention
		if utils.ContainsObjectID(userIDs, mention) {
			userIDs = append(userIDs, mention)
		}
	}

	// Fetch user data
	var users []*datastructure.User
	if err := cache.Find(ctx, "users", "", bson.M{
		"_id": bson.M{
			"$in": userIDs,
		},
	}, users); err != nil {
		return b, err
	}

	b.Notification.Content.Users = users
	return b, nil
}

// Write the notification to database, creating it if it doesn't exist, or updating the existing one
func (b NotificationBuilder) Write(ctx context.Context) error {
	upsert := true

	// Create new Object ID if this is a new notification
	if b.Notification.ID.IsZero() {
		b.Notification.ID = primitive.NewObjectID()
	}

	if _, err := mongo.Database.Collection("notifications").UpdateByID(ctx, b.Notification.ID, bson.M{
		"$set": b.Notification,
	}, &options.UpdateOptions{
		Upsert: &upsert,
	}); err != nil {
		log.WithError(err).Error("mongo")
		return err
	}

	return nil
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

// Append a User Mention to the notification
func (b NotificationBuilder) AddUserMentionPart(user primitive.ObjectID) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type:    datastructure.NotificationContentMessagePartTypeUserMention,
		Mention: &user,
	})

	return b
}

// Append a Emote Mention to the notification
func (b NotificationBuilder) AddEmoteMentionPart(emote primitive.ObjectID) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type:    datastructure.NotificationContentMessagePartTypeEmoteMention,
		Mention: &emote,
	})

	return b
}

// Append a Role Mention to the notification
func (b NotificationBuilder) AddRoleMentionPart(role primitive.ObjectID) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type:    datastructure.NotificationContentMessagePartTypeRoleMention,
		Mention: &role,
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
		Notification: datastructure.Notification{
			ReadBy:      []primitive.ObjectID{},
			TargetUsers: []primitive.ObjectID{},
			TargetRoles: []primitive.ObjectID{},
			Content: datastructure.NotificationContent{
				Title:        "System Message",
				MessageParts: []datastructure.NotificationContentMessagePart{},
			},
		},
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

var Notifications notifications = notifications{}
