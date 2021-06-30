package query_resolvers

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	mongocache "github.com/SevenTV/ServerGo/src/mongo/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	api_proxy "github.com/SevenTV/ServerGo/src/server/api/v2/proxy"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/selection"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type QueryResolver struct{}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var searchRegex = regexp.MustCompile(`[.*+?^${}()|[\\]\\\\]`)

type SelectedField struct {
	Name     string
	Children map[string]*SelectedField
}

func GenerateSelectedFieldMap(ctx context.Context, max int) (*SelectedField, bool) {
	var loop func(fields []*selection.SelectedField, localDepth int) (map[string]*SelectedField, int)
	loop = func(fields []*selection.SelectedField, localDepth int) (map[string]*SelectedField, int) {
		if len(fields) == 0 {
			return nil, localDepth
		}
		m := map[string]*SelectedField{}
		localDepth++
		maxD := localDepth
		for _, f := range fields {
			if maxD > max {
				return nil, maxD
			}
			children, d := loop(f.SelectedFields, localDepth)
			m[f.Name] = &SelectedField{
				Name:     f.Name,
				Children: children,
			}
			if d > maxD {
				maxD = d
			}
		}
		return m, maxD
	}
	children, depth := loop(graphql.SelectedFieldsFromContext(ctx), 0)
	return &SelectedField{
		Name:     "query",
		Children: children,
	}, depth > max
}

func (*QueryResolver) AuditLogs(ctx context.Context, args struct {
	Page  int32
	Limit *int32
	Types *[]int32
}) ([]*auditResolver, error) {
	var logs []*datastructure.AuditLog

	// Find audit logs
	var limit int32 = 150
	if args.Limit != nil {
		limit = *args.Limit
	}

	query := bson.M{}
	if args.Types != nil && len(*args.Types) > 0 {
		query["type"] = bson.M{
			"$in": *args.Types,
		}
	}
	fmt.Println("query:", query)

	if err := cache.Find(ctx, "audit", "", query, &logs, &options.FindOptions{
		Limit: utils.Int64Pointer(int64(math.Min(250, float64(limit)))),
		Sort: bson.M{
			"_id": -1,
		},
	}); err != nil {
		log.WithError(err).Error("mongo")
		return nil, err
	}
	fmt.Println("logs", logs)

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	resolvers := make([]*auditResolver, len(logs))
	for i, l := range logs {
		resolver, err := GenerateAuditResolver(ctx, l, field.Children)
		if err != nil {
			log.WithError(err).Error("GenerateAuditResolver")
			return nil, err
		}
		if resolver == nil {
			continue
		}

		resolvers[i] = resolver
	}

	return resolvers, nil
}

func (*QueryResolver) User(ctx context.Context, args struct{ ID string }) (*UserResolver, error) {
	isMe := args.ID == "@me" // Handle @me (current authenticated user)
	user := &datastructure.User{}
	var id *primitive.ObjectID
	if isMe {
		// Get current user from context if @me
		if u, ok := ctx.Value(utils.UserKey).(*datastructure.User); isMe && u != nil && ok {
			user = u
			id = &u.ID
		}
		if isMe && user.ID.IsZero() { // Handle error: current user requested but request was unauthenticated
			return nil, fmt.Errorf("Cannot request @me while unauthenticated")
		}
	} else if !primitive.IsValidObjectID(args.ID) {
		if err := cache.FindOne(ctx, "users", "", bson.M{
			"$or": bson.A{
				bson.M{"login": strings.ToLower(args.ID)},
				bson.M{"id": args.ID},
			},
		}, user); err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil
			}
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}
	} else {
		if hexId, err := primitive.ObjectIDFromHex(args.ID); err == nil {
			id = &hexId
			user = nil
		} else {
			return nil, nil
		}
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	return GenerateUserResolver(ctx, user, id, field.Children)
}

func (*QueryResolver) Role(ctx context.Context, args struct{ ID string }) (*RoleResolver, error) {
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	return GenerateRoleResolver(ctx, nil, &id, field.Children)
}

func (*QueryResolver) Emote(ctx context.Context, args struct{ ID string }) (*EmoteResolver, error) {
	id, err := primitive.ObjectIDFromHex(args.ID)
	if err != nil {
		return nil, nil
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	resolver, err := GenerateEmoteResolver(ctx, nil, &id, field.Children)
	if err != nil {
		return nil, err
	}
	// Get actor user
	usr, usrOK := ctx.Value(utils.UserKey).(*datastructure.User)
	// Verify actor permissions
	if utils.BitField.HasBits(int64(resolver.v.Visibility), int64(datastructure.EmoteVisibilityPrivate)) && (!usrOK || usr.ID != resolver.v.OwnerID) {
		if !usrOK || !usr.HasPermission(datastructure.RolePermissionEmoteEditAll) {
			return nil, resolvers.ErrUnknownEmote
		}
	}

	return resolver, nil
}

func (*QueryResolver) Emotes(ctx context.Context, args struct{ List []string }) (*[]*EmoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	ids := mongo.HexIDSliceToObjectID(args.List)
	emotes := []*datastructure.Emote{}
	if len(args.List) > 0 {
		if err := cache.Find(ctx, "emotes", "", bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		}, &emotes); err != nil {
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}
	}

	resolvers := make([]*EmoteResolver, len(emotes))
	if len(emotes) > 0 {
		for i, e := range emotes {
			resolvers[i], _ = GenerateEmoteResolver(ctx, e, nil, field.Children)
		}
	}

	return &resolvers, nil
}

func (*QueryResolver) SearchEmotes(ctx context.Context, args struct {
	Query       string
	Page        *int32
	PageSize    *int32
	Limit       *int32
	GlobalState *string
	SortBy      *string
	SortOrder   *int32
	Channel     *string
	SubmittedBy *string
	Filter      *EmoteSearchFilter
}) ([]*EmoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	// Define limit
	// This is how many emotes can be searched in one request at most
	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > resolvers.QueryLimit {
		return nil, resolvers.ErrQueryLimit
	}

	// Get the query parameter, used to search for specific emote names or tags
	query := strings.Trim(args.Query, " ")
	hasQuery := len(query) > 0
	lQuery := fmt.Sprintf("(?i)%s", strings.ToLower(searchRegex.ReplaceAllString(query, "\\\\$0")))

	// Get actor user
	usr, _ := ctx.Value(utils.UserKey).(*datastructure.User)

	// Create aggregation
	opts := options.Aggregate()
	emotes := []*datastructure.Emote{}
	match := bson.M{
		"status": datastructure.EmoteStatusLive,
	}
	if args.Channel != nil {
		var targetChannel *datastructure.User
		// Find user and get their emotes
		if err := cache.FindOne(ctx, "users", "", bson.M{"login": args.Channel}, &targetChannel); err == nil {
			match["_id"] = bson.M{"$in": targetChannel.EmoteIDs}
		}
	}

	if usr == nil || !usr.HasPermission(datastructure.RolePermissionEmoteEditAll) {
		var usrID primitive.ObjectID
		if usr != nil {
			usrID = usr.ID
		}

		match["$and"] = bson.A{
			bson.M{"$or": bson.A{
				bson.M{"visibility": bson.M{"$bitsAllClear": int32(datastructure.EmoteVisibilityPrivate | datastructure.EmoteVisibilityUnlisted)}},
				bson.M{"owner": usrID},
			}},
		}
	}

	if args.Filter != nil {
		// Handle visibility filter
		if args.Filter.Visibility != nil {
			match["visibility"] = bson.M{"$bitsAllSet": *args.Filter.Visibility}
		}

		// Handle width range filter
		if args.Filter.WidthRange != nil {
			if len(*args.Filter.WidthRange) != 2 { // Error if the length wasn't 2
				return nil, fmt.Errorf("filter.width_range must be a list with 2 integers, but the length given was %d", len(*args.Filter.WidthRange))
			}

			list := *args.Filter.WidthRange
			min := list[0]
			max := list[1]
			if max < min {
				return nil, fmt.Errorf("the max value cannot be smaller than the minimum value")
			}

			match["width.3"] = bson.M{
				"$gte": min,
				"$lte": max,
			}
		}
	}

	// Pagination
	page := int64(1)
	if args.Page != nil && *args.Page > 1 {
		page = int64(*args.Page)
	}
	pageSize := limit
	if args.PageSize != nil {
		pageSize = int64(*args.PageSize)
	}

	// Define aggregation pipeline
	pipeline := mongo.Pipeline{
		bson.D{
			bson.E{
				Key:   "$match",
				Value: match,
			},
		}, // Match query
		bson.D{
			bson.E{
				Key:   "$skip",
				Value: (page - 1) * pageSize,
			},
		}, // Paginate
		bson.D{
			bson.E{
				Key:   "$limit",
				Value: math.Max(0, math.Min(float64(pageSize), float64(limit))),
			},
		}, // Set limit
	}

	// Get sorting direction
	var order int32 = 1
	if args.SortOrder != nil {
		order = *args.SortOrder
	}
	if order > 1 {
		return nil, resolvers.ErrInvalidSortOrder
	}
	if order == 1 {
		order = -1
	} else if order == 0 {
		order = 1
	}

	// Handle sorting
	if args.SortBy != nil {
		sortBy := *args.SortBy
		switch sortBy {
		// Popularity Sort - Channels Added
		case "popularity":
			// Sort by channel count
			pipeline = append(mongo.Pipeline{
				bson.D{
					bson.E{
						Key: "$sort",
						Value: bson.D{
							bson.E{
								Key:   "channel_count",
								Value: -order,
							},
						},
					},
				},
			}, pipeline...)

		// Creation Date Sort
		case "age":
			pipeline = append(mongo.Pipeline{
				bson.D{
					bson.E{
						Key: "$sort",
						Value: bson.D{
							bson.E{
								Key:   "_id",
								Value: order,
							},
						},
					},
				},
			}, pipeline...)
		}
	}

	// If a query is specified, add sorting
	if hasQuery {
		match["$or"] = bson.A{
			bson.M{
				"name": bson.M{
					"$regex": lQuery,
				},
			},
			bson.M{
				"tags": bson.M{
					"$regex": lQuery,
				},
			},
		}
	}

	// If global state is specified, filter global emotes
	if args.GlobalState != nil {
		globalState := *args.GlobalState
		switch globalState {
		case "only": // Only: query only global emotes
			match["visibility"] = bson.M{"$bitsAllSet": int32(datastructure.EmoteVisibilityGlobal)}
		case "hide": // Hide: omit global emotes from query
			match["visibility"] = bson.M{"$bitsAllClear": int32(datastructure.EmoteVisibilityGlobal)}
		}
	}

	// Determine the full collection size
	f := ctx.Value(utils.RequestCtxKey).(*fiber.Ctx) // Fiber context

	// Count documents in the collection
	count, err := cache.GetCollectionSize(ctx, "emotes", match)
	if err != nil {
		return nil, err
	}

	f.Response().Header.Add("X-Collection-Size", fmt.Sprint(count))

	// Query the DB
	cur, err := mongo.Database.Collection("emotes").Aggregate(ctx, pipeline, opts)

	if err == nil {
		err = cur.All(ctx, &emotes)
	}
	if err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	// Resolve emotes
	resolvers := make([]*EmoteResolver, len(emotes))
	for i, e := range emotes {
		resolvers[i], err = GenerateEmoteResolver(ctx, e, nil, field.Children)
		if err != nil {
			return nil, err
		}
	}
	return resolvers, nil
}

type EmoteSearchFilter struct {
	WidthRange *[]int32
	Visibility *int32
}

func (*QueryResolver) ThirdPartyEmotes(ctx context.Context, args struct {
	Providers []string
	Channel   string
	Global    *bool
}) (*[]*EmoteResolver, error) {
	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	// TEMPORARY FIX:
	// FORCE FFZ EMOTES.
	// THIS SHOULD BE REMOVED WHEN THE WEB EXTENSION IS FIXED
	if !utils.Contains(args.Providers, "FFZ") {
		args.Providers = append(args.Providers, "FFZ")
	}

	// Query foreign APIs for requested third party emotes
	var emotes []*datastructure.Emote
	var globalEmotes []*datastructure.Emote
	for _, p := range args.Providers {
		switch p {
		case "BTTV": // Handle BTTV Provider
			if args.Channel != "" {
				if bttv, err := api_proxy.GetChannelEmotesBTTV(ctx, args.Channel); err == nil { // Find channel bttv emotes
					emotes = append(emotes, bttv...)
				}
			}
			if args.Global != nil && *args.Global {
				if bttvG, err := api_proxy.GetGlobalEmotesBTTV(ctx); err == nil { // Find global bttv emotes
					globalEmotes = append(globalEmotes, bttvG...)
				}
			}
		case "FFZ": // Handle FFZ Provider
			if args.Channel != "" {
				if ffz, err := api_proxy.GetChannelEmotesFFZ(ctx, args.Channel); err == nil { // Find channel FFZ emotes
					emotes = append(emotes, ffz...)
				}
			}
			if args.Global != nil && *args.Global {
				if ffzG, err := api_proxy.GetGlobalEmotesFFZ(ctx); err == nil { // Find global ffz emotes
					globalEmotes = append(globalEmotes, ffzG...)
				}
			}
		}
	}
	for _, e := range globalEmotes {
		e.Visibility = datastructure.EmoteVisibilityGlobal
	}
	emotes = append(emotes, globalEmotes...)

	// Create emote resolvers to return
	result := make([]*EmoteResolver, len(emotes))
	for i, emote := range emotes {
		resolver, _ := GenerateEmoteResolver(ctx, emote, nil, field.Children)
		if resolver == nil {
			continue
		}

		result[i] = resolver
	}

	return &result, nil
}

func (*QueryResolver) SearchUsers(ctx context.Context, args struct {
	Query string
	Page  *int32
	Limit *int32
}) ([]*UserResolver, error) {
	usr, _ := ctx.Value(utils.UserKey).(*datastructure.User)
	if usr == nil || !usr.HasPermission(datastructure.RolePermissionManageUsers) {
		return nil, resolvers.ErrAccessDenied
	}

	field, failed := GenerateSelectedFieldMap(ctx, resolvers.MaxDepth)
	if failed {
		return nil, resolvers.ErrDepth
	}

	limit := int64(20)
	if args.Limit != nil {
		limit = int64(*args.Limit)
	}
	if limit > resolvers.QueryLimit {
		return nil, resolvers.ErrQueryLimit
	}

	// Pagination
	page := int64(1)
	if args.Page != nil && *args.Page > 1 {
		page = int64(*args.Page)
	}

	query := strings.Trim(args.Query, " ")
	lQuery := fmt.Sprintf("(?i)%s", strings.ToLower(searchRegex.ReplaceAllString(query, "\\\\$0")))

	opts := options.Find().SetSort(bson.M{
		"login": 1,
	}).SetLimit(limit).SetSkip((page - 1) * limit)

	users := []*datastructure.User{}

	match := bson.M{
		"login": bson.M{
			"$regex": lQuery,
		},
	}
	cur, err := mongo.Database.Collection("users").Find(ctx, match, opts)
	if err == nil {
		err = cur.All(ctx, &users)
	}
	if err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	}

	// Determine the full collection size
	{
		f := ctx.Value(utils.RequestCtxKey).(*fiber.Ctx) // Fiber context

		// Count documents in the collection
		count, err := cache.GetCollectionSize(ctx, "users", match)
		if err != nil {
			return nil, err
		}

		f.Response().Header.Add("X-Collection-Size", fmt.Sprint(count))
	}

	resolvers := make([]*UserResolver, len(users))
	for i, e := range users {
		resolvers[i], err = GenerateUserResolver(ctx, e, nil, field.Children)
		if err != nil {
			return nil, err
		}
	}
	return resolvers, nil
}

func (*QueryResolver) FeaturedBroadcast(ctx context.Context) (string, error) {
	channel := redis.Client.Get(ctx, "meta:featured_broadcast").Val()
	if channel == "" {
		return "", fmt.Errorf("No Featured Broadcast")
	}

	stream, err := api_proxy.GetTwitchStreams(ctx, channel)
	if err != nil {
		log.WithError(err).WithField("channel", channel).Error("query could not get live status of featured broadcast")
		return "", err
	}

	if len(stream.Data) == 0 || stream.Data[0].Type != "live" {
		return "", fmt.Errorf("featured broadcast not live")
	}

	return channel, nil
}

func (*QueryResolver) Meta(ctx context.Context) (*datastructure.Meta, error) {
	pipe := redis.Client.Pipeline()
	announce := pipe.Get(ctx, "meta:announcement")
	feat := pipe.Get(ctx, "meta:featured_broadcast")
	_, _ = pipe.Exec(ctx)
	if err := announce.Err(); err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
	}
	if err := feat.Err(); err != nil && err != redis.ErrNil {
		log.WithError(err).Error("redis")
	}

	cachedRoles := mongocache.CachedRoles.([]datastructure.Role)
	roles := []string{}
	for _, r := range cachedRoles {
		b, err := json.Marshal(r)
		if err != nil {
			continue
		}

		roles = append(roles, utils.B2S(b))
	}

	return &datastructure.Meta{
		Announcement:      announce.Val(),
		FeaturedBroadcast: feat.Val(),
		Roles:             roles,
	}, nil
}
