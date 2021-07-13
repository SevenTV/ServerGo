package emotes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/gographics/imagick.v3/imagick"

	"github.com/SevenTV/ServerGo/src/cache"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

const chunkSize = 1024 * 1024

var (
	errInternalServer = []byte(`{"status":500,"message":"internal server error"}`)
	errInvalidRequest = `{"status":400,"message":"%s"}`
	errAccessDenied   = `{"status":403,"message":"%s"}`
)

func GetEmoteRoute(router fiber.Router) {
	// Get Emote
	router.Get("/:emote", middleware.RateLimitMiddleware("get-emote", 30, 6*time.Second),
		func(c *fiber.Ctx) error {
			// Parse Emote ID
			id, err := primitive.ObjectIDFromHex(c.Params("emote"))
			if err != nil {
				return restutil.MalformedObjectId().Send(c)
			}

			// Fetch emote data
			var emote datastructure.Emote
			if err := cache.FindOne(c.Context(), "emotes", "", bson.M{
				"_id": id,
			}, &emote); err != nil {
				if err == mongo.ErrNoDocuments {
					return restutil.ErrUnknownEmote().Send(c)
				}
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			// Fetch emote owner
			var owner *datastructure.User
			if err := cache.FindOne(c.Context(), "users", "", bson.M{
				"_id": emote.OwnerID,
			}, &owner); err != nil {
				if err != mongo.ErrNoDocuments {
					return restutil.ErrInternalServer().Send(c, err.Error())
				}
			}

			response := restutil.CreateEmoteResponse(&emote, owner)

			b, err := json.Marshal(&response)
			if err != nil {
				return restutil.ErrInternalServer().Send(c, err.Error())
			}

			return c.Send(b)
		})

	// Convert an emote in the CDN from WEBP or other format into PNG
	rl := configure.Config.GetIntSlice("limits.route.emote-convert")
	var m func(*fiber.Ctx) error
	if rl != nil {
		m = middleware.RateLimitMiddleware("emote-convert", int32(rl[0]), time.Millisecond*time.Duration(rl[1]))
	}

	router.Use("/:emote/convert.gif", m)
	router.Get("/:emote/convert.gif", func(c *fiber.Ctx) error {
		emoteID := c.Params("emote") // Get the emote ID parameter

		// Create a new magick wand
		wand := imagick.NewMagickWand()
		defer wand.Destroy()

		// Get CDN URL
		url := utils.GetCdnURL(emoteID, 3)

		// Download the image from the CDN
		res, err := http.Get(url)
		if err != nil {
			log.WithError(err).Error("http")
			return restutil.ErrAccessDenied().Send(c, fmt.Sprintf("Couldn't get file: %v", err.Error()))
		}
		defer res.Body.Close()
		if res.StatusCode != 200 { // Check status
			return restutil.ErrAccessDenied().Send(c, fmt.Sprintf("CDN returned non-200 status code (%d %v)", res.StatusCode, res.Status))
		}

		// Read response body and append to a byte slice
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return restutil.ErrAccessDenied().Send(c, fmt.Sprintf("Failed to read file: %v", err.Error()))
		}

		// Add image to the magick wand
		if err = wand.ReadImageBlob(b); err != nil {
			log.WithError(err).Error("could not decode image")
			return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Couldn't decode image: %v", err.Error()))
		}

		// Convert & stream back to client
		wand.SetIteratorIndex(0)
		wand.SetImageFormat("gif")
		wand.ResetIterator()
		c.Set("Content-Type", "image/gif")
		return c.SendStream(bytes.NewReader(wand.GetImagesBlob()))
	})
}

type OEmbedData struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
}
