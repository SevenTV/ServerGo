package query_resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	log "github.com/sirupsen/logrus"
)

type NotificationResolver struct {
	ctx context.Context
	v   *datastructure.Notification

	fields map[string]*SelectedField
}

func GenerateNotificationResolver(ctx context.Context, notification *datastructure.Notification, fields map[string]*SelectedField) (*NotificationResolver, error) {
	return &NotificationResolver{
		ctx,
		notification,
		fields,
	}, nil
}

func (r *NotificationResolver) ID() string {
	return r.v.ID.Hex()
}

func (r *NotificationResolver) Announcement() bool {
	return r.v.Announcement
}

func (r *NotificationResolver) Title() string {
	return r.v.Title
}

func (r *NotificationResolver) Timestamp() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *NotificationResolver) ReadAt() *string {
	if r.v.ReadAt.IsZero() {
		return nil
	}

	date := r.v.ReadAt.Format(time.RFC3339)
	return &date
}

func (r *NotificationResolver) MessageParts() []*messagePart {
	parts := make([]*messagePart, len(r.v.MessageParts))

	for i, v := range r.v.MessageParts {
		pType := int32(v.Type)
		pData := ""
		if v.Type != datastructure.NotificationMessagePartTypeText {
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

func (r *NotificationResolver) Users() ([]*UserResolver, error) {
	builder := actions.Notifications.CreateFrom(*r.v)

	users := builder.Notification.Users
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

func (r *NotificationResolver) Emotes() ([]*EmoteResolver, error) {
	builder := actions.Notifications.CreateFrom(*r.v)

	users := builder.Notification.Emotes
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

func (r *NotificationResolver) Read() bool {
	return r.v.Read
}

type messagePart struct {
	Type int32  `json:"type"`
	Data string `json:"data"`
}
