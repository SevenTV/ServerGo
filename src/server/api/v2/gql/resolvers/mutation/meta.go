package mutation_resolvers

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	"github.com/SevenTV/ServerGo/src/utils"
)

func (*MutationResolver) EditApp(ctx context.Context, args struct {
	Properties struct {
		FeaturedBroadcast *string
	}
}) (*response, error) {
	usr, ok := ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok {
		return nil, resolvers.ErrLoginRequired
	}
	if !usr.HasPermission(datastructure.RolePermissionEditApplicationMeta) {
		return nil, resolvers.ErrAccessDenied
	}

	// Edit featured broadcast
	if args.Properties.FeaturedBroadcast != nil {
		redis.Client.Set(ctx, "meta:featured_broadcast", *args.Properties.FeaturedBroadcast, 0)
	}

	return &response{}, nil
}

type MetaInput struct {
	FeaturedBroadcast string `json:"featured_broadcast"`
}
