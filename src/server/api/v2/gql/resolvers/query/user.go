package query_resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/api/actions"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql/resolvers"
	api_proxy "github.com/SevenTV/ServerGo/src/server/api/v2/proxy"
	"github.com/SevenTV/ServerGo/src/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.mongodb.org/mongo-driver/bson/primitive"

	log "github.com/sirupsen/logrus"
)

type UserResolver struct {
	ctx context.Context
	v   *datastructure.User

	fields map[string]*SelectedField
}

func GenerateUserResolver(ctx context.Context, user *datastructure.User, userID *primitive.ObjectID, fields map[string]*SelectedField) (*UserResolver, error) {
	if user == nil || user.Login == "" {
		user = &datastructure.User{
			Role: datastructure.DefaultRole,
		}
		if err := cache.FindOne(ctx, "users", "", bson.M{
			"_id": userID,
		}, user); err != nil {
			if err != mongo.ErrNoDocuments {
				log.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}
			return nil, nil
		}
	}

	if user == nil {
		return nil, nil
	}

	usr, usrValid := ctx.Value(utils.UserKey).(*datastructure.User)
	actorCanEdit := false
	if usr != nil {
		actorCanEdit = usr.ID == user.ID

		if !actorCanEdit {
			for _, e := range user.EditorIDs {
				if e == usr.ID {
					actorCanEdit = true
					break
				}
			}
		}
	}

	if v, ok := fields["owned_emotes"]; ok && user.OwnedEmotes == nil {
		user.OwnedEmotes = &[]*datastructure.Emote{}
		if err := cache.Find(ctx, "emotes", fmt.Sprintf("owner:%s", user.ID.Hex()), bson.M{
			"owner":  user.ID,
			"status": datastructure.EmoteStatusLive,
		}, user.OwnedEmotes); err != nil {
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}
		ems := *user.OwnedEmotes
		ids := make([]primitive.ObjectID, len(ems))
		emotes := make(map[primitive.ObjectID]*datastructure.Emote, len(ems))
		for i, e := range ems {
			e.Owner = user
			ids[i] = e.ID
			emotes[e.ID] = e
		}
		if _, ok := v.Children["audit_entries"]; ok {
			logs := []*datastructure.AuditLog{}
			if err := cache.Find(ctx, "audit", fmt.Sprintf("user:%s:owned_emotes", user.ID.Hex()), bson.M{
				"target.id": bson.M{
					"$in": ids,
				},
				"target.type": "emotes",
			}, &logs); err != nil {
				log.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}

			for _, l := range logs {
				e := emotes[*l.Target.ID]
				var d []*datastructure.AuditLog
				if e.AuditEntries != nil {
					d = *e.AuditEntries
				}
				d = append(d, l)
				e.AuditEntries = &d
			}
		}
	}

	if v, ok := fields["emotes"]; ok && user.Emotes == nil {
		if len(user.EmoteIDs) == 0 {
			user.Emotes = &[]*datastructure.Emote{}
		} else {
			user.Emotes = &[]*datastructure.Emote{}
			if err := cache.Find(ctx, "emotes", fmt.Sprintf("user:%s:emotes", user.ID.Hex()), bson.M{
				"_id": bson.M{
					"$in": user.EmoteIDs,
				},
			}, user.Emotes); err != nil {
				log.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}
			ems := *user.Emotes
			ids := make([]primitive.ObjectID, len(ems))
			emotes := make(map[primitive.ObjectID]*datastructure.Emote, len(ems))
			for i, e := range ems {
				ids[i] = e.ID
				emotes[e.ID] = e
			}
			if _, ok := v.Children["audit_entries"]; ok {
				logs := []*datastructure.AuditLog{}
				if err := cache.Find(ctx, "audit", "", bson.M{
					"target.id": bson.M{
						"$in": ids,
					},
					"target.type": "emotes",
				}, &logs); err != nil {
					log.WithError(err).Error("mongo")
					return nil, resolvers.ErrInternalServer
				}
				for _, l := range logs {
					e := emotes[*l.Target.ID]
					var d []*datastructure.AuditLog
					if e.AuditEntries != nil {
						d = *e.AuditEntries
					}
					d = append(d, l)
					e.AuditEntries = &d
				}
			}
		}
	}

	if _, ok := fields["editors"]; ok && user.Editors == nil {
		user.Editors = &[]*datastructure.User{}
		if err := cache.Find(ctx, "users", fmt.Sprintf("user:%s:editors", user.ID.Hex()), bson.M{
			"_id": bson.M{
				"$in": utils.Ternary(len(user.EditorIDs) > 0, user.EditorIDs, []primitive.ObjectID{}).([]primitive.ObjectID),
			},
		}, user.Editors); err != nil {
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}
	}

	if _, ok := fields["editor_in"]; ok && user.EditorIn == nil {
		user.EditorIn = &[]*datastructure.User{}

		if cur, err := mongo.Collection(mongo.CollectionNameUsers).Find(ctx, bson.M{
			"editors": user.ID,
		}); err != nil {
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		} else {
			if err = cur.All(ctx, user.EditorIn); err != nil {
				log.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}
		}
	}

	if v, ok := fields["reports"]; ok && usrValid && (usr.Rank != datastructure.UserRankAdmin && usr.Rank != datastructure.UserRankModerator) && user.Reports == nil {
		user.Reports = &[]*datastructure.Report{}
		if err := cache.Find(ctx, "users", fmt.Sprintf("user:%s:reports", user.ID.Hex()), bson.M{
			"target.id":   user.ID,
			"target.type": "users",
		}, user.EditorIn); err != nil {
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}

		_, query := v.Children["reporter"]

		reports := *user.Reports
		reportMap := map[primitive.ObjectID][]*datastructure.Report{}
		for _, r := range reports {
			r.UTarget = user
			if r.ReporterID != nil && query {
				reportMap[*r.ReporterID] = append(reportMap[*r.ReporterID], r)
			}
		}
		if query {
			ids := []primitive.ObjectID{}
			for k := range reportMap {
				ids = append(ids, k)
			}
			reporters := []*datastructure.User{}

			if err := cache.Find(ctx, "users", "", bson.M{
				"_id": bson.M{
					"$in": ids,
				},
			}, &reporters); err != nil {
				log.WithError(err).Error("mongo")
				return nil, resolvers.ErrInternalServer
			}
			for _, u := range reporters {
				for _, r := range reportMap[u.ID] {
					r.Reporter = u
				}
			}
		}
	}

	if _, ok := fields["bans"]; ok && usrValid && usr.HasPermission(datastructure.RolePermissionBanUsers) && user.Bans == nil {
		user.Bans = &[]*datastructure.Ban{}
		res, err := mongo.Collection(mongo.CollectionNameBans).Find(ctx, bson.M{
			"user_id": user.ID,
		})
		if err != nil {
			log.WithError(err).Error("mongo")
			return nil, resolvers.ErrInternalServer
		}

		_ = res.All(ctx, user.Bans)
	}

	if _, ok := fields["notifications"]; ok && usrValid && actorCanEdit {
		// Find notifications readable by this user
		pipeline := mongo.Pipeline{
			bson.D{ // Step 1: Match only readstates where the target is the user
				bson.E{
					Key: "$match",
					Value: bson.M{
						"target": user.ID,
					},
				},
			},
			bson.D{
				bson.E{
					Key: "$sort",
					Value: bson.M{
						"_id": -1,
					},
				},
			},
			bson.D{
				bson.E{
					Key:   "$limit",
					Value: 50,
				},
			},
			bson.D{ // Step 2: Find the target notification from the other collection
				bson.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "notifications", // Target the collection containing notification data
						"localField":   "notification",  // Use the notification field, which is the ID of the notification
						"foreignField": "_id",           // Match with foreign collection's ObjectID
						"as":           "notification",  // Output as "notification" field
					},
				},
			},
			bson.D{ // Step 3: Unwind the array of notifications (but this is ID match, therefore there is only 1)
				bson.E{
					Key:   "$unwind",
					Value: "$notification",
				},
			},
			bson.D{ // Step 4: Add the "read" field from the notification read state into the notification object
				bson.E{
					Key: "$addFields",
					Value: bson.M{
						"notification.read":    "$read",
						"notification.read_at": "$read_at",
					},
				},
			},
			bson.D{ // Step 5: Replace the root input with the notification that now has the readstate information :tf:
				bson.E{
					Key:   "$replaceWith",
					Value: "$notification",
				},
			},
		}
		cur, err := mongo.Collection(mongo.CollectionNameNotificationsRead).Aggregate(ctx, pipeline)
		if err != nil {
			return nil, err
		}
		if err := cur.All(ctx, &user.Notifications); err != nil {
			return nil, err
		}
	}

	if _, ok := fields["notification_count"]; ok && usrValid && actorCanEdit {
		// Get count of notifications
		count, err := cache.GetCollectionSize(ctx, "notifications_read", bson.M{
			"target": user.ID,
			"read":   false,
		})
		if err != nil {
			return nil, err
		}

		user.NotificationCount = &count
	}

	r := &UserResolver{
		ctx:    ctx,
		v:      user,
		fields: fields,
	}
	return r, nil
}

func (r *UserResolver) ID() string {
	if r.v.ID.IsZero() {
		return ""
	}

	return r.v.ID.Hex()
}

func (r *UserResolver) Email() *string {
	if u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User); ok && (r.v.ID == u.ID || u.HasPermission(datastructure.RolePermissionManageUsers)) {
		return &r.v.Email
	} else { // Hide the email address if
		s := "<hidden>"
		return &s
	}
}

func (r *UserResolver) Rank() int32 {
	return r.v.Rank
}

func (r *UserResolver) Description() string {
	return r.v.Description
}

func (r *UserResolver) Role() (*RoleResolver, error) {
	ub, err := actions.Users.With(r.ctx, r.v)
	if err != nil {
		return nil, err
	}

	role := ub.GetRole()
	res, err := GenerateRoleResolver(r.ctx, &role, &role.ID, nil)
	if err != nil {
		log.WithError(err).Error("generation")
		return nil, resolvers.ErrInternalServer
	}

	if res == nil {
		return GenerateRoleResolver(r.ctx, datastructure.DefaultRole, nil, r.fields["role"].Children)
	}

	return res, nil
}

func (r *UserResolver) EmoteIDs() []string {
	ids := make([]string, len(r.v.EmoteIDs))
	for i, id := range r.v.EmoteIDs {
		ids[i] = id.Hex()
	}
	return ids
}

func (r *UserResolver) EmoteAliases() [][]string {
	result := make([][]string, len(r.v.EmoteAlias))

	i := 0
	for id, name := range r.v.EmoteAlias {
		result[i] = []string{id, name}
		i++
	}

	return result
}

func (r *UserResolver) EditorIDs() []string {
	ids := make([]string, len(r.v.EditorIDs))
	for i, id := range r.v.EditorIDs {
		ids[i] = id.Hex()
	}
	return ids
}

func (r *UserResolver) CreatedAt() string {
	return r.v.ID.Timestamp().Format(time.RFC3339)
}

func (r *UserResolver) Editors() ([]*UserResolver, error) {
	editors := *r.v.Editors
	result := []*UserResolver{}
	for _, e := range editors {
		r, err := GenerateUserResolver(r.ctx, e, nil, r.fields["editors"].Children)
		if err != nil {
			log.WithError(err).Error("generation")
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) EditorIn() ([]*UserResolver, error) {
	editors := *r.v.EditorIn
	result := []*UserResolver{}
	for _, e := range editors {
		r, err := GenerateUserResolver(r.ctx, e, nil, r.fields["editor_in"].Children)
		if err != nil {
			log.WithError(err).Error("generation")
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) Emotes() ([]*EmoteResolver, error) {
	if r.v.Emotes == nil {
		return nil, nil
	}
	emotes := datastructure.UserUtil.GetAliasedEmotes(r.v)
	result := []*EmoteResolver{}

	zeroWidthOK := r.v.HasPermission(datastructure.RolePermissionUseZerowidthEmote)
	for _, e := range emotes {
		if !zeroWidthOK && utils.BitField.HasBits(int64(e.Visibility), int64(datastructure.EmoteVisibilityZerowidth)) {
			continue // Skip if the emote is zero-width and user lacks permission
		}

		r, err := GenerateEmoteResolver(r.ctx, e, nil, r.fields["emotes"].Children)
		if err != nil {
			log.WithError(err).Error("generation")
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) OwnedEmotes() ([]*EmoteResolver, error) {
	emotes := *r.v.OwnedEmotes
	result := []*EmoteResolver{}
	for _, e := range emotes {
		r, err := GenerateEmoteResolver(r.ctx, e, nil, r.fields["owned_emotes"].Children)
		if err != nil {
			log.WithError(err).Error("generation")
			return nil, resolvers.ErrInternalServer
		}
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

func (r *UserResolver) ThirdPartyEmotes() ([]*EmoteResolver, error) {
	var emotes []*datastructure.Emote
	if bttv, err := api_proxy.GetChannelEmotesBTTV(r.ctx, r.v.Login); err == nil { // Find channel bttv emotes
		emotes = append(emotes, bttv...)
	}
	if ffz, err := api_proxy.GetChannelEmotesFFZ(r.ctx, r.v.Login); err == nil { // Find channel FFZ emotes
		emotes = append(emotes, ffz...)
	}

	result := make([]*EmoteResolver, len(emotes))
	for i, emote := range emotes {
		resolver, _ := GenerateEmoteResolver(r.ctx, emote, nil, r.fields["third_party_emotes"].Children)
		if resolver == nil {
			continue
		}

		result[i] = resolver
	}

	return result, nil
}

func (r *UserResolver) TwitchID() string {
	return r.v.TwitchID
}

func (r *UserResolver) DisplayName() string {
	return r.v.DisplayName
}

func (r *UserResolver) Login() string {
	return r.v.Login
}

func (r *UserResolver) BroadcasterType() string {
	return r.v.BroadcasterType
}

func (r *UserResolver) ProfileImageURL() string {
	return r.v.ProfileImageURL
}

func (r *UserResolver) Reports() (*[]*reportResolver, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok || (u.Rank != datastructure.UserRankAdmin && u.Rank != datastructure.UserRankModerator) {
		return nil, resolvers.ErrAccessDenied
	}

	if r.v.Reports == nil {
		return nil, nil
	}

	e := *r.v.Reports
	reports := make([]*reportResolver, len(e))
	var err error
	for i, l := range e {
		reports[i], err = GenerateReportResolver(r.ctx, l, r.fields["reports"].Children)
		if err != nil {
			return nil, err
		}
	}
	return &reports, nil
}

func (r *UserResolver) Bans() (*[]*banResolver, error) {
	u, ok := r.ctx.Value(utils.UserKey).(*datastructure.User)
	if !ok || !u.HasPermission(datastructure.RolePermissionBanUsers) {
		return nil, resolvers.ErrAccessDenied
	}

	if r.v.Bans == nil {
		return nil, nil
	}

	e := *r.v.Bans
	bans := make([]*banResolver, len(e))
	var err error
	for i, l := range e {
		bans[i], err = GenerateBanResolver(r.ctx, l, r.fields["bans"].Children)
		if err != nil {
			return nil, err
		}
	}
	return &bans, nil
}

func (r *UserResolver) Banned() bool {
	return redis.Client.HGet(r.ctx, "user:bans", r.v.ID.Hex()).Val() != ""
}

func (r *UserResolver) AuditEntries() (*[]*auditResolver, error) {
	var logs []*datastructure.AuditLog
	if cur, err := mongo.Collection(mongo.CollectionNameAudit).Find(r.ctx, bson.M{
		"target.type": "users",
		"target.id":   r.v.ID,
	}, &options.FindOptions{
		Sort: bson.M{
			"_id": -1,
		},
		Limit: utils.Int64Pointer(30),
	}); err != nil {
		log.WithError(err).Error("mongo")
		return nil, resolvers.ErrInternalServer
	} else {
		err := cur.All(r.ctx, &logs)
		if err != nil && err != mongo.ErrNoDocuments {
			log.WithError(err).Error("mongo")
			return nil, err
		}
	}

	resolvers := make([]*auditResolver, len(logs))
	for i, l := range logs {
		if l == nil {
			continue
		}

		resolver, err := GenerateAuditResolver(r.ctx, l, r.fields)
		if err != nil {
			log.WithError(err).Error("GenerateAuditResolver")
			continue
		}

		resolvers[i] = resolver
	}

	return &resolvers, nil
}

func (r *UserResolver) EmoteSlots() int32 {
	return r.v.GetEmoteSlots()
}

// Get user's folloer count
func (r *UserResolver) FollowerCount() int32 {
	count, err := api_proxy.GetTwitchFollowerCount(r.ctx, r.v.TwitchID)
	if err != nil {
		return 0
	}

	return count
}

// Get user's live broadcast, if any
func (r *UserResolver) Broadcast() (*datastructure.Broadcast, error) {
	var stream *datastructure.Broadcast
	if streams, err := api_proxy.GetTwitchStreams(r.ctx, r.v.Login); err != nil {
		return nil, err
	} else if len(streams.Data) > 0 {
		s := streams.Data[0]
		stream = &datastructure.Broadcast{
			ID:           s.ID,
			Title:        s.Title,
			ThumbnailURL: s.ThumbnailURL,
			ViewerCount:  s.ViewerCount,
			Type:         s.Type,
			GameName:     s.GameName,
			GameID:       s.GameID,
			Language:     s.Language,
			Tags:         s.TagIDs,
			Mature:       s.IsMature,
			StartedAt:    s.StartedAt.Format(time.RFC3339),
			UserID:       s.UserID,
		}
	} else {
		return nil, nil
	}

	return stream, nil
}

func (r *UserResolver) Notifications() ([]*NotificationResolver, error) {
	// Transform all notifications to builders
	notifications := []actions.NotificationBuilder{}
	for _, n := range r.v.Notifications {
		if n == nil {
			continue
		}

		notifications = append(notifications, actions.Notifications.CreateFrom(*n))
	}

	if len(notifications) == 0 {
		return []*NotificationResolver{}, nil
	}

	var (
		mentionedUserIDs  []primitive.ObjectID
		mentionedUsers    []*datastructure.User
		mentionedEmoteIDs []primitive.ObjectID
		mentionedEmotes   []*datastructure.Emote
		tempmap           map[primitive.ObjectID]bool
		resolvers         = []*NotificationResolver{}
	)

	for i, n := range notifications {
		n, tempmap = n.GetMentionedUsers(r.ctx)
		for k := range tempmap {
			if utils.ContainsObjectID(mentionedUserIDs, k) { // Skip if the user is already added to mentions
				continue
			}
			mentionedUserIDs = append(mentionedUserIDs, k)
		}
		n, tempmap = n.GetMentionedEmotes(r.ctx)
		for k := range tempmap {
			if utils.ContainsObjectID(mentionedEmoteIDs, k) { // Skip if the emote is already added to mentions
				continue
			}
			mentionedEmoteIDs = append(mentionedEmoteIDs, k)
		}
		notifications[i] = n
	}

	if len(mentionedUserIDs) > 0 {
		if err := cache.Find(r.ctx, "users", "", bson.M{
			"_id": bson.M{
				"$in": mentionedUserIDs,
			},
		}, &mentionedUsers); err != nil {
			log.WithError(err).Error("mongo")
		}
	}
	if len(mentionedEmoteIDs) > 0 {
		if err := cache.Find(r.ctx, "emotes", "", bson.M{
			"_id": bson.M{
				"$in": mentionedEmoteIDs,
			},
		}, &mentionedEmotes); err != nil {
			log.WithError(err).Error("mongo")
		}
	}

	for _, n := range notifications {
		for _, u := range mentionedUsers {
			if !utils.ContainsObjectID(n.MentionedUsers, u.ID) {
				continue
			}
			n.Notification.Users = append(n.Notification.Users, u)
		}
		for _, e := range mentionedEmotes {
			if !utils.ContainsObjectID(n.MentionedEmotes, e.ID) {
				continue
			}
			n.Notification.Emotes = append(n.Notification.Emotes, e)
		}

		notify := n.Notification
		resolver, err := GenerateNotificationResolver(r.ctx, &notify, r.fields)
		if err != nil {
			log.WithError(err).Error("GenerateNotificationResolver")
			continue
		}
		resolvers = append(resolvers, resolver)
	}

	return resolvers, nil
}

func (r *UserResolver) NotificationCount() int32 {
	if r.v.NotificationCount == nil {
		return 0
	}

	return int32(*r.v.NotificationCount)
}
