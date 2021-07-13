package emotes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
// errInternalServer = []byte(`{"status":500,"message":"internal server error"}`)
// errInvalidRequest = `{"status":400,"message":"%s"}`
// errAccessDenied   = `{"status":403,"message":"%s"}`
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

	g := router.Group("/")
	// Convert an emote in the CDN from WEBP or other format into PNG
	rl := configure.Config.GetIntSlice("limits.route.emote-convert")
	if rl != nil {
		g.Use(middleware.RateLimitMiddleware("emote-convert", int32(rl[0]), time.Millisecond*time.Duration(rl[1])))
	}

	g.Get("/:emote/:size.gif", func(c *fiber.Ctx) error {
		emoteID := c.Params("emote") // Get the emote ID parameter
		s := c.Params("size")
		if s[len(s)-1] == 'x' {
			s = s[:len(s)-1]
		}

		size, err := strconv.ParseUint(s, 10, 8)
		if err != nil || size > 4 || size < 1 {
			return c.SendStatus(404)
		}

		// Create a new magick wand
		wand := imagick.NewMagickWand()
		defer wand.Destroy()

		// Get CDN URL
		url := utils.GetCdnURL(emoteID, uint8(size))

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

		if string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
			if string(b[:8]) == string([]byte{137, 80, 78, 71, 13, 10, 26, 10}) { // is PNG
				c.Set("Content-Type", "image/png")
				c.Set("Cache-Control", "public, max-age=15552000")
				return c.Send(b)
			} else {
				if string(b[:3]) != "GIF" { // is not webp, is not png, is not gif
					return c.SendStatus(415)
				}
				// is GIF
				c.Set("Content-Type", "image/gif")
				c.Set("Cache-Control", "public, max-age=15552000")
				return c.Send(b)
			}
		}

		isAnimated := false
		for i := range b[:len(b)-4] {
			if utils.B2S(b[i:i+4]) == "ANIM" {
				isAnimated = true
				break
			}
		}
		if !isAnimated {
			c.Set("Content-Type", "image/webp")
			c.Set("Cache-Control", "public, max-age=15552000")
			return c.Send(b)
		}

		// Add image to the magick wand
		if err = wand.ReadImageBlob(b); err != nil {
			log.WithError(err).Error("could not decode image")
			return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Couldn't decode image: %v", err.Error()))
		}

		// Convert & stream back to client
		wand.SetIteratorIndex(0)
		if err := wand.SetImageFormat("gif"); err != nil {
			log.WithError(err).Error("could not decode image")
			return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Couldn't decode image: %v", err.Error()))
		}
		wand.ResetIterator()

		c.Set("Content-Type", "image/gif")
		c.Set("Cache-Control", "public, max-age=15552000")

		return c.Send(wand.GetImageBlob())
	})
}

type OEmbedData struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	AuthorURL    string `json:"author_url"`
	ProviderName string `json:"provider_name"`
	ProviderURL  string `json:"provider_url"`
}
