package mutation_resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (*MutationResolver) MarkNotificationsRead(ctx context.Context, args struct {
	NotificationIDs []string
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Parse IDs
	var ids []primitive.ObjectID
	for _, id := range args.NotificationIDs {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}

		ids = append(ids, oid)
	}

	// Get the requested notification
	res, err := mongo.Collection(mongo.CollectionNameNotificationsRead).UpdateMany(ctx, bson.M{
		"notification": bson.M{
			"$in": ids,
		},
		"target": usr.ID,
	}, bson.M{
		"$set": bson.M{
			"read":    true,
			"read_at": time.Now(),
		},
	})
	if err != nil {
		return nil, err
	}

	return &response{
		Status:  200,
		Message: fmt.Sprintf("Marked %d notifications as read", res.ModifiedCount),
	}, nil
}
