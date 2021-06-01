package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type webSocketHelpers struct {
	subscriberCallersUserEmotes map[string]*eventCallback
	callbackMtx                 sync.Mutex
}

type eventCallback struct {
	mtx       sync.Mutex
	userID    string
	callbacks map[uuid.UUID]func(emoteSubscriptionResult)
	ctx       context.Context
}

func (e *eventCallback) start(ctx context.Context) {
	rCh := make(chan []byte)
	key := fmt.Sprintf("users:%v:emotes", e.userID)
	sub := redis.Subscribe(ctx, rCh, key)
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return

		case b := <-rCh:
			// Get new emote list
			var d redis.PubSubPayloadUserEmotes
			if err := json.Unmarshal(b, &d); err != nil {
				log.Errorf("websocket, err=%v", err)
				continue
			}

			// Get full emote objects for added
			var emote *datastructure.Emote
			id, err := primitive.ObjectIDFromHex(d.ID)
			if err != nil {
				continue
			}

			if err := cache.FindOne(ctx, "emotes", "", bson.M{"_id": id}, &emote); err != nil {
				continue
			}
			urls := datastructure.GetEmoteURLs(*emote)
			emote.URLs = urls
			emote.Provider = "7TV"
			result := emoteSubscriptionResult{
				Emote: &datastructure.Emote{
					ID:         emote.ID,
					Provider:   emote.Provider,
					Visibility: emote.Visibility,
					Mime:       emote.Mime,
					Name:       emote.Name,
					URLs:       emote.URLs,
				},
				Removed: d.Removed,
				Channel: e.userID,
				Actor:   d.Actor,
			}

			e.mtx.Lock()
			if len(e.callbacks) == 0 {
				e.mtx.Unlock()
				return
			}
			for _, fn := range e.callbacks {
				go fn(result)
			}
			e.mtx.Unlock()
		}
	}
}

/*
* SUBSCRIBER CHANNEL: User Emotes
*
* It listens for updates on a user's channel emotes and forwards it to all active callers
 */
func (h *webSocketHelpers) SubscriberChannelUserEmotes(ctx context.Context, userID string, cb func(emoteSubscriptionResult)) {
	c := ctx.Value(WebSocketConnKey).(*Conn)
	h.callbackMtx.Lock()
	v, ok := h.subscriberCallersUserEmotes[userID]
	if !ok {
		v = &eventCallback{
			callbacks: make(map[uuid.UUID]func(emoteSubscriptionResult)),
		}
		go func() {
			vctx, cancel := context.WithCancel(context.Background())
			defer func() {
				if err := recover(); err != nil {
					log.WithField("err", err).Error("panic")
				}
				h.callbackMtx.Lock()
				delete(h.subscriberCallersUserEmotes, userID)
				h.callbackMtx.Unlock()
				cancel()
			}()
			v.start(vctx)
		}()
		h.subscriberCallersUserEmotes[userID] = v
	}
	v.mtx.Lock()
	v.callbacks[c.Stat.UUID] = cb
	v.mtx.Unlock()
	h.callbackMtx.Unlock()

	select {
	case <-ctx.Done():
	case <-v.ctx.Done():
	}
	v.mtx.Lock()
	delete(v.callbacks, c.Stat.UUID)
	v.mtx.Unlock()
}
