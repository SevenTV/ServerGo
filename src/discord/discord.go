package discord

import (
	"fmt"
	"strconv"

	"github.com/SevenTV/ServerGo/src/configure"
	dgo "github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// An empty Discord session for executing webhooks
var d, _ = dgo.New(fmt.Sprintf("Bot %v", configure.Config.GetString("discord.bot_token")))
var webhooks = make(map[string]webhookInfo)

type webhookInfo struct {
	ID    string
	Token string
}

func init() {
	s := configure.Config.GetStringSlice("discord.webhook.activity")
	if len(s) == 2 {
		webhooks["activity"] = webhookInfo{
			ID:    s[0],
			Token: s[1],
		}
	}

	s = configure.Config.GetStringSlice("discord.webhooks.alerts")
	if len(s) == 2 {
		webhooks["alerts"] = webhookInfo{
			ID:    s[0],
			Token: s[1],
		}
	}
}

func toIntColor(s string) int {
	i, err := strconv.ParseInt(s, 16, 32)
	if err != nil {
		return 0
	}

	return int(i)
}

func SendWebhook(name string, params *dgo.WebhookParams) *dgo.Message {
	wh, ok := webhooks[name]
	if !ok || (wh.ID == "" || wh.Token == "") {
		// Discord is disabled.
		return nil
	}

	if msg, err := d.WebhookExecute(wh.ID, wh.Token, true, params); err == nil {
		return msg
	} else {
		log.WithError(err).Error("discord, SendWebhook")
	}

	return nil
}

var Discord = d
