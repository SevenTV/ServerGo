package discord

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/utils"
	dgo "github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func SendEmoteCreate(emote datastructure.Emote, actor datastructure.User) {
	_ = SendWebhook("activity", &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** 🆕 emote [%s](%v) created by %s", emote.Name, utils.GetEmotePageURL(emote.ID.Hex()), actor.DisplayName),
		Embeds: []*dgo.MessageEmbed{
			{
				Title: emote.Name,
				Author: &dgo.MessageEmbedAuthor{
					URL:     utils.GetUserPageURL(actor.ID.Hex()),
					IconURL: actor.ProfileImageURL,
					Name:    actor.DisplayName,
				},
				Image: &dgo.MessageEmbedImage{
					URL: utils.GetEmoteImageURL(emote.ID.Hex()),
				},
				Color: toIntColor("24e575"),
			},
		},
	})
}

func SendEmoteEdit(emote datastructure.Emote, actor datastructure.User, logs []*datastructure.AuditLogChange, reason *string) {
	fields := make([]*dgo.MessageEmbedField, len(logs))
	for i, ch := range logs {
		fields[i] = &dgo.MessageEmbedField{
			Name:   ch.Key,
			Value:  fmt.Sprintf("%s ➡️ %s", fmt.Sprint(ch.OldValue), fmt.Sprint(ch.NewValue)),
			Inline: true,
		}
	}
	if reason != nil && len(*reason) > 0 {
		fields = append(fields, &dgo.MessageEmbedField{
			Name:  "Reason",
			Value: *reason,
		})
	}

	_ = cache.FindOne(context.Background(), "users", "", bson.M{
		"_id": emote.OwnerID,
	}, &emote.Owner)

	ownerName := datastructure.DeletedUser.DisplayName
	if emote.Owner != nil {
		ownerName = emote.Owner.DisplayName
	}
	_ = SendWebhook("activity", &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** ✏️ emote [%s](%v) edited by [%s](%v)", emote.Name, utils.GetEmotePageURL(emote.ID.Hex()), actor.DisplayName, utils.GetUserPageURL(actor.ID.Hex())),
		Embeds: []*dgo.MessageEmbed{
			{
				Title:       emote.Name,
				Description: fmt.Sprintf("by %v", ownerName),
				Author: &dgo.MessageEmbedAuthor{
					URL:     utils.GetUserPageURL(actor.ID.Hex()),
					IconURL: actor.ProfileImageURL,
					Name:    actor.DisplayName,
				},
				Thumbnail: &dgo.MessageEmbedThumbnail{
					URL: utils.GetEmoteImageURL(emote.ID.Hex()),
				},
				Fields: fields,
				Color:  toIntColor("e3b464"),
			},
		},
	})
}

func SendEmoteDelete(emote datastructure.Emote, actor datastructure.User, reason string) {
	_ = SendWebhook("activity", &dgo.WebhookParams{
		Content: fmt.Sprintf("**[activity]** ❌ emote [%s](%v) deleted by [%s](%v)", emote.Name, utils.GetEmotePageURL(emote.ID.Hex()), actor.DisplayName, utils.GetUserPageURL(actor.ID.Hex())),
		Embeds: []*dgo.MessageEmbed{
			{
				Author: &dgo.MessageEmbedAuthor{
					URL:     utils.GetUserPageURL(actor.ID.Hex()),
					IconURL: actor.ProfileImageURL,
					Name:    actor.DisplayName,
				},
				Description: fmt.Sprintf("Reason: %s", reason),
			},
		},
	})
}

func SendEmoteMerge(emote1 datastructure.Emote, emote2 datastructure.Emote, actor datastructure.User, channels int32, reason string) {
	_ = SendWebhook("activity", &dgo.WebhookParams{
		Content: fmt.Sprintf(
			"**[activity]** 🔀 [%v](%v) merged the emote [%v](%v) into [%v](%v)",
			actor.DisplayName, utils.GetUserPageURL(actor.ID.Hex()),
			emote1.Name, utils.GetEmotePageURL(emote1.ID.Hex()),
			emote2.Name, utils.GetEmotePageURL(emote2.ID.Hex()),
		),
		Embeds: []*dgo.MessageEmbed{
			{
				Description: fmt.Sprintf("Reason: %v", reason),
				Color:       16725715,
				Author: &dgo.MessageEmbedAuthor{
					URL:     utils.GetUserPageURL(actor.ID.Hex()),
					IconURL: actor.ProfileImageURL,
					Name:    actor.DisplayName,
				},
				Thumbnail: &dgo.MessageEmbedThumbnail{
					URL: utils.GetEmoteImageURL(emote2.ID.Hex()),
				},
				Fields: []*dgo.MessageEmbedField{
					{Name: "Channels Switched", Value: fmt.Sprint(channels)},
				},
			},
		},
	})
}

func SendPopularityCheckUpdateNotice(wg *sync.WaitGroup) {
	_ = SendWebhook("activity", &dgo.WebhookParams{
		Content: "**[routine]** ⚙️ updating emote popularities...",
	})

	wg.Wait()

	_ = SendWebhook("activity", &dgo.WebhookParams{
		Content: "**[routine]** ⚙️ successfully updated emote popularities!",
	})
}

func SendPanic(output string) {
	_ = SendWebhook("alerts", &dgo.WebhookParams{
		Content: fmt.Sprintf("**[PANIC]** NODE: **%v** | POD: **%v** | TIME: **%v**", configure.NodeName, configure.PodName, time.Now().UTC().Format("Monday, January 2 15:04:05 -0700 MST 2006")),
		Embeds: []*dgo.MessageEmbed{
			{
				Color:       16728642,
				Description: output[:int(math.Min(2000, float64(len(output))))],
				Fields: []*dgo.MessageEmbedField{
					{Name: "Node", Value: configure.NodeName, Inline: true},
					{Name: "Pod", Value: configure.PodName, Inline: true},
				},
			},
		},
	})
}

func SendServiceDown(serviceName string) {
	pingRole := configure.Config.GetString("discord.webhooks.sysadmin_role")

	_ = SendWebhook("alerts", &dgo.WebhookParams{
		Content: fmt.Sprintf("🟥 **[HEALTH CHECK FAILURE]** **%v DOWN!** <a:FEELSWAYTOODANKMAN:774004379878424607>🤜🏼🔔<@&%v>", strings.ToUpper(serviceName), configure.Config.GetString("discord.webhooks.sysadmin_role")),
		Embeds: []*dgo.MessageEmbed{
			{
				Color: 16728642,
				Fields: []*dgo.MessageEmbedField{
					{Name: "Node", Value: configure.NodeName, Inline: true},
					{Name: "Pod", Value: configure.PodName, Inline: true},
				},
			},
		},
		AllowedMentions: &dgo.MessageAllowedMentions{
			Roles: utils.Ternary(pingRole != "", []string{pingRole}, []string{}).([]string),
		},
	})
}

func SendServiceRestored(serviceName string) {
	_ = SendWebhook("alerts", &dgo.WebhookParams{
		Content: fmt.Sprintf("✅ **[SERVICE RESTORED]** %v OK <a:FeelsDankCube:854955723221762058>", strings.ToUpper(serviceName)),
		Embeds: []*dgo.MessageEmbed{
			{
				Color: 3319890,
				Fields: []*dgo.MessageEmbedField{
					{Name: "Node", Value: configure.NodeName, Inline: true},
					{Name: "Pod", Value: configure.PodName, Inline: true},
				},
			},
		},
	})
}
