package mutation_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (*MutationResolver) MergeEmote(ctx context.Context, args struct {
	OldID  string
	NewID  string
	Reason string
}) (*query_resolvers.EmoteResolver, error) {
	// Get the actor user
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}

	// Check permissions
	if !usr.HasPermission(datastructure.RolePermissionEmoteEditAll) {
		return nil, resolvers.ErrAccessDenied
	}

	// Parse emote IDs
	var (
		oldID primitive.ObjectID
		newID primitive.ObjectID
		err   error
	)
	if oldID, err = primitive.ObjectIDFromHex(args.OldID); err != nil {
		log.WithError(err).Error("failed to merge emotes")
		return nil, err
	}
	if newID, err = primitive.ObjectIDFromHex(args.NewID); err != nil {
		log.WithError(err).Error("failed to merge emotes")
		return nil, err
	}

	emote, err := actions.Emotes.MergeEmote(ctx, actions.MergeEmoteOptions{
		Actor:  usr,
		OldID:  oldID,
		NewID:  newID,
		Reason: args.Reason,
	})
	if err != nil {
		log.WithError(err).Error("failed to merge emotes")
		return nil, err
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	return query_resolvers.GenerateEmoteResolver(ctx, emote, &emote.ID, field.Children)
}
