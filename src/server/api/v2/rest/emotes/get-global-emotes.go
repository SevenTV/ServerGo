package emotes

import (
	"encoding/json"
	"time"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

func GetGlobalEmotes(router fiber.Router) {
	router.Get("/global", middleware.RateLimitMiddleware("get-global-emotes", 25, 16*time.Second),
		func(c *fiber.Ctx) error {
			var emotes []*datastructure.Emote
			if err := cache.Find(c.Context(), "emotes", "", bson.M{
				"visibility": bson.M{
					"$bitsAllSet": datastructure.EmoteVisibilityGlobal,
				},
			}, &emotes); err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			response := make([]restutil.EmoteResponse, len(emotes))
			for i, emote := range emotes {
				response[i] = restutil.CreateEmoteResponse(emote, nil)
			}

			j, err := json.Marshal(response)
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			return c.Send(j)
		})
}
