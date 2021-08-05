package redis

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Publish to a redis channel
func Publish(ctx context.Context, channel string, data interface{}) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cmd := Client.Publish(ctx, channel, j)
	if cmd.Err() != nil {
		return nil
	}

	return nil
}

// Subscribe to a channel on Redis
func Subscribe(ctx context.Context, ch chan []byte, subscribeTo ...string) {
	subsMtx.Lock()
	defer subsMtx.Unlock()
	localSub := &redisSub{ch}
	for _, e := range subscribeTo {
		if _, ok := subs[e]; !ok {
			_ = sub.Subscribe(ctx, e)
		}
		subs[e] = append(subs[e], localSub)
	}

	go func() {
		<-ctx.Done()
		subsMtx.Lock()
		defer subsMtx.Unlock()
		for _, e := range subscribeTo {
			for i, v := range subs[e] {
				if v == localSub {
					if i != len(subs[e])-1 {
						subs[e][i] = subs[e][len(subs[e])-1]
					}
					subs[e] = subs[e][:len(subs[e])-1]
					if len(subs[e]) == 0 {
						delete(subs, e)
						if err := sub.Unsubscribe(context.Background(), e); err != nil {
							log.WithError(err).Error("failed to unsubscribe")
						}
					}
					break
				}
			}
		}
	}()
}

type PubSubPayloadUserEmotes struct {
	Removed bool   `json:"removed"`
	ID      string `json:"id"`
	Actor   string `json:"actor"`
}

type EventApiV1ChannelEmotes struct {
	Channel string                        `json:"channel"`
	EmoteID string                        `json:"emote_id"`
	Name    string                        `json:"name"`
	Action  string                        `json:"action"`
	Actor   string                        `json:"actor"`
	Emote   *EventApiV1ChannelEmotesEmote `json:"emote"`
}

type EventApiV1ChannelEmotesEmote struct {
	Name       string                            `json:"name"`
	Visibility int32                             `json:"visibility"`
	MIME       string                            `json:"mime"`
	Tags       []string                          `json:"tags"`
	Width      [4]int16                          `json:"width"`
	Height     [4]int16                          `json:"height"`
	Animated   bool                              `json:"animated"`
	Owner      EventApiV1ChannelEmotesEmoteOwner `json:"owner"`
}

type EventApiV1ChannelEmotesEmoteOwner struct {
	ID          string `json:"id"`
	TwitchID    string `json:"twitch_id"`
	DisplayName string `json:"display_name"`
	Login       string `json:"login"`
}
