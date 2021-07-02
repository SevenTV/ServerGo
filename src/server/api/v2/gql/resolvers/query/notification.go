package query_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	log "github.com/sirupsen/logrus"
)

type notificationResolver struct {
	ctx context.Context
	v   *datastructure.Notification

	fields map[string]*SelectedField
}

func GenerateNotificationResolver(ctx context.Context, notification *datastructure.Notification, fields map[string]*SelectedField) (*notificationResolver, error) {
	return &notificationResolver{
		ctx,
		notification,
		fields,
	}, nil
}

func (r *notificationResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *notificationResolver) Announcement() bool {
	return r.v.Announcement
}

func (r *notificationResolver) Title() string {
	return r.v.Content.Title
}

func (r *notificationResolver) MessageParts() []*messagePart {
	parts := make([]*messagePart, len(r.v.Content.MessageParts))

	for i, v := range r.v.Content.MessageParts {
		pType := int32(v.Type)
		pData := ""
		if v.Type != datastructure.NotificationContentMessagePartTypeText {
			pData = v.Mention.Hex()
		} else if v.Text != nil {
			pData = *v.Text
		} else {
			log.WithError(fmt.Errorf("Bad Notification Message Part")).
				WithField("notification_id", r.v.ID).
				WithField("part_index", i).
				Error("notification")

			continue
		}

		p := messagePart{
			Type: pType,
			Data: pData,
		}
		parts[i] = &p
	}

	return parts
}

func (r *notificationResolver) ReadBy() []string {
	result := make([]string, len(r.v.ReadBy))
	for i, v := range r.v.ReadBy {
		result[i] = v.Hex()
	}

	return result
}

func (r *notificationResolver) Users() ([]*UserResolver, error) {
	builder := actions.Notifications.CreateFrom(*r.v)

	users := builder.Notification.Content.Users
	resolvers := make([]*UserResolver, len(users))
	for i, v := range users {
		resolver, err := GenerateUserResolver(r.ctx, v, &v.ID, r.fields)
		if err != nil {
			return nil, err
		}

		resolvers[i] = resolver
	}

	return resolvers, nil
}

func (r *notificationResolver) Emotes() ([]*EmoteResolver, error) {
	builder := actions.Notifications.CreateFrom(*r.v)

	users := builder.Notification.Content.Emotes
	resolvers := make([]*EmoteResolver, len(users))
	for i, v := range users {
		resolver, err := GenerateEmoteResolver(r.ctx, v, &v.ID, r.fields)
		if err != nil {
			return nil, err
		}

		resolvers[i] = resolver
	}

	return resolvers, nil
}

type messagePart struct {
	Type int32  `json:"type"`
	Data string `json:"data"`
}
