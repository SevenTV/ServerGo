package actions

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type notifications struct{}

type NotificationBuilder struct {
	Notification    datastructure.Notification
	MentionedUsers  []primitive.ObjectID
	MentionedEmotes []primitive.ObjectID
	MentionedRoles  []primitive.ObjectID
}

// GetMentionedUsers: Get the data of mentioned users in the notification's message parts
func (b NotificationBuilder) GetMentionedUsers(ctx context.Context) (NotificationBuilder, map[primitive.ObjectID]bool) {
	userIDs := make(map[primitive.ObjectID]bool)
	for _, part := range b.Notification.Content.MessageParts { // Check message parts for user mentions
		if part.Type != datastructure.NotificationContentMessagePartTypeUserMention {
			continue
		}
		if part.Mention == nil {
			continue
		}

		// Append unique user IDs to slice
		mention := *part.Mention
		if _, ok := userIDs[mention]; !ok {
			userIDs[mention] = true
			b.MentionedUsers = append(b.MentionedUsers, mention)
		}
	}
	return b, userIDs
}

// GetMentionedEmotes: Get the data of mentioned emotes in the notification's message parts
func (b NotificationBuilder) GetMentionedEmotes(ctx context.Context) (NotificationBuilder, map[primitive.ObjectID]bool) {
	emoteIDs := make(map[primitive.ObjectID]bool)
	for _, part := range b.Notification.Content.MessageParts { // Check message parts for emote mentions
		if part.Type != datastructure.NotificationContentMessagePartTypeEmoteMention {
			continue
		}
		if part.Mention == nil {
			continue
		}

		// Append unique user IDs to slice
		mention := *part.Mention
		if _, ok := emoteIDs[mention]; !ok {
			emoteIDs[mention] = true
			b.MentionedEmotes = append(b.MentionedEmotes, mention)
		}
	}
	return b, emoteIDs
}

// Write: Write the notification to database, creating it if it doesn't exist, or updating the existing one
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

// SetTitle: Set the Notification's Title
func (b NotificationBuilder) SetTitle(title string) NotificationBuilder {
	b.Notification.Content.Title = title

	return b
}

// AddTextMessagePart: Append a Text part to the notification
func (b NotificationBuilder) AddTextMessagePart(text string) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type: datastructure.NotificationContentMessagePartTypeText,
		Text: &text,
	})

	return b
}

// AddUserMentionPart: Append a User Mention to the notification
func (b NotificationBuilder) AddUserMentionPart(user primitive.ObjectID) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type:    datastructure.NotificationContentMessagePartTypeUserMention,
		Mention: &user,
	})

	return b
}

// AddEmoteMentionPart: Append a Emote Mention to the notification
func (b NotificationBuilder) AddEmoteMentionPart(emote primitive.ObjectID) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type:    datastructure.NotificationContentMessagePartTypeEmoteMention,
		Mention: &emote,
	})

	return b
}

// AddRoleMentionPart: Append a Role Mention to the notification
func (b NotificationBuilder) AddRoleMentionPart(role primitive.ObjectID) NotificationBuilder {
	b.Notification.Content.MessageParts = append(b.Notification.Content.MessageParts, datastructure.NotificationContentMessagePart{
		Type:    datastructure.NotificationContentMessagePartTypeRoleMention,
		Mention: &role,
	})

	return b
}

// AddTargetUsers: Add one or more users who may read this notification
func (b NotificationBuilder) AddTargetUsers(userIDs ...primitive.ObjectID) NotificationBuilder {
	b.Notification.TargetUsers = append(b.Notification.TargetUsers, userIDs...)

	return b
}

// AddTargetRoles: Add one or more roles that may allow their members to read this notification
func (b NotificationBuilder) AddTargetRoles(roleIDs ...primitive.ObjectID) NotificationBuilder {
	b.Notification.TargetRoles = append(b.Notification.TargetRoles, roleIDs...)

	return b
}

// MarkAsAnnouncement: Mark this notification as an announcement, meaning all users will be able to read it
// regardless of the selected targets
func (b NotificationBuilder) MarkAsAnnouncement() NotificationBuilder {
	b.Notification.Announcement = true

	return b
}

// SetReadBy: The users who've read this notification
func (b NotificationBuilder) SetReadBy(userIDs ...primitive.ObjectID) NotificationBuilder {
	b.Notification.ReadBy = append(b.Notification.ReadBy, userIDs...)

	return b
}

// Create: Get a NotificationBuilder
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

// CreateFrom: Get a NotificationBuilder populated with an existing notification
func (*notifications) CreateFrom(notification datastructure.Notification) NotificationBuilder {
	builder := NotificationBuilder{
		Notification: notification,
	}

	return builder
}

var Notifications notifications = notifications{}
