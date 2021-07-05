package mutation_resolvers

import (
	"context"
	"fmt"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	query_resolvers "github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers/query"
	"github.com/SevenTV/ServerGo/src/utils"
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
	)
	if id, err := primitive.ObjectIDFromHex(args.OldID); err != nil {
		return nil, err
	} else {
		oldID = id
	}
	if id, err := primitive.ObjectIDFromHex(args.NewID); err != nil {
		return nil, err
	} else {
		newID = id
	}

	emote, err := actions.Emotes.MergeEmote(ctx, actions.MergeEmoteOptions{
		Actor:  usr,
		OldID:  oldID,
		NewID:  newID,
		Reason: args.Reason,
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	field, failed := query_resolvers.GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	return query_resolvers.GenerateEmoteResolver(ctx, emote, &emote.ID, field.Children)
}
