package v2

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/api"
	"github.com/SevenTV/ServerGo/src/jwt"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/gofiber/fiber/v2"

	"github.com/pasztorpisti/qs"

	log "github.com/sirupsen/logrus"
)

type TwitchTokenResp struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

type TwitchCallback struct {
	Challenge    string                     `json:"challenge"`
	Subscription TwitchCallbackSubscription `json:"subscription"`
	Event        map[string]interface{}     `json:"event"`
}

type TwitchCallbackSubscription struct {
	ID        string                  `json:"id"`
	Status    string                  `json:"status"`
	Type      string                  `json:"type"`
	Version   string                  `json:"version"`
	Condition map[string]interface{}  `json:"condition"`
	Transport TwitchCallbackTransport `json:"transport"`
}

type csrfJWT struct {
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

type TwitchCallbackTransport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

func Twitch(app fiber.Router) fiber.Router {
	twitch := app.Group("/auth")

	twitch.Get("/", func(c *fiber.Ctx) error {
		csrfToken, err := utils.GenerateRandomString(64)
		if err != nil {
			log.WithError(err).Error("secure bytes")
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		scopes := []string{}

		scopes = append(scopes, "user:read:email")

		cookieStore, err := jwt.Sign(csrfJWT{
			State:     csrfToken,
			CreatedAt: time.Now(),
		})
		if err != nil {
			log.WithError(err).Error("jwt")
			return c.Status(500).JSON(&fiber.Map{
				"message": "Internal server error.",
				"status":  500,
			})
		}

		c.Cookie(&fiber.Cookie{
			Name:     "csrf_token",
			Value:    cookieStore,
			Expires:  time.Now().Add(time.Hour),
			Domain:   configure.Config.GetString("cookie_domain"),
			Secure:   configure.Config.GetBool("cookie_secure"),
			HTTPOnly: true,
		})

		params, _ := qs.Marshal(map[string]string{
			"client_id":     configure.Config.GetString("twitch_client_id"),
			"redirect_uri":  configure.Config.GetString("twitch_redirect_uri"),
			"response_type": "code",
			"scope":         strings.Join(scopes, " "),
			"state":         csrfToken,
		})

		u := fmt.Sprintf("https://id.twitch.tv/oauth2/authorize?%s", params)

		return c.Redirect(u)
	})

	twitch.Get("/callback", func(c *fiber.Ctx) error {
		twitchToken := c.Query("state")

		if twitchToken == "" {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, missing state paramater.",
			})
		}

		token := strings.Split(c.Cookies("csrf_token"), ".")
		if len(token) != 3 {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from cookies.",
			})
		}

		pl := &csrfJWT{}
		if err := jwt.Verify(token, pl); err != nil {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from cookies.",
			})
		}

		if pl.CreatedAt.Before(time.Now().Add(-time.Hour)) {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Expired.",
			})
		}

		if twitchToken != pl.State {
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, csrf_token token missmatch.",
			})
		}

		c.Cookie(&fiber.Cookie{
			Name:     "csrf_token",
			MaxAge:   -1,
			Domain:   configure.Config.GetString("cookie_domain"),
			Secure:   configure.Config.GetBool("cookie_secure"),
			HTTPOnly: true,
		})

		code := c.Query("code")

		params, _ := qs.Marshal(map[string]string{
			"client_id":     configure.Config.GetString("twitch_client_id"),
			"client_secret": configure.Config.GetString("twitch_client_secret"),
			"redirect_uri":  configure.Config.GetString("twitch_redirect_uri"),
			"code":          code,
			"grant_type":    "authorization_code",
		})

		req, err := http.NewRequestWithContext(c.Context(), "POST", fmt.Sprintf("https://id.twitch.tv/oauth2/token?%s", params), nil)
		if err != nil {
			log.WithError(err).Error("twitch")
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.WithError(err).Error("twitch")
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Error("ioutils")
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		tokenResp := TwitchTokenResp{}
		if err := json.Unmarshal(data, &tokenResp); err != nil {
			log.WithError(err).WithField("data", utils.B2S(data)).Error("twitch")
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert code to access token.",
			})
		}

		users, err := api.GetUsers(c.Context(), tokenResp.AccessToken, nil, nil)
		if err != nil || len(users) != 1 {
			log.WithError(err).WithField("resp", users).WithField("token", tokenResp).Error("twitch")
			return c.Status(400).JSON(&fiber.Map{
				"status":  400,
				"message": "Invalid response from twitch, failed to convert access token to user account. (" + err.Error() + ")",
			})
		}

		user := users[0]
		after := options.After
		doc := mongo.Collection(mongo.CollectionNameUsers).FindOneAndUpdate(c.Context(), bson.M{
			"id": user.ID,
		}, bson.M{
			"$set": user,
		}, &options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
		})

		mongoUser := &datastructure.User{}
		if doc.Err() != nil {
			if doc.Err() == mongo.ErrNoDocuments {
				mongoUser = &datastructure.User{
					TwitchID:        user.ID,
					DisplayName:     user.DisplayName,
					Login:           user.Login,
					ProfileImageURL: user.ProfileImageURL,
					Email:           user.Email,
					Rank:            datastructure.UserRankDefault,
					Description:     user.Description,
					CreatedAt:       user.CreatedAt,
					OfflineImageURL: user.OfflineImageURL,
					BroadcasterType: user.BroadcasterType,
					ViewCount:       int32(user.ViewCount),
					EmoteIDs:        []primitive.ObjectID{},
					EditorIDs:       []primitive.ObjectID{},
					TokenVersion:    "1",
				}
				res, err := mongo.Collection(mongo.CollectionNameUsers).InsertOne(c.Context(), mongoUser)
				if err != nil {
					log.WithError(err).Error("mongo")
					return c.Status(500).JSON(&fiber.Map{
						"status":  500,
						"message": "Failed to create new account.",
					})
				}

				var ok bool
				mongoUser.ID, ok = res.InsertedID.(primitive.ObjectID)
				if !ok {
					log.WithField("resp", res.InsertedID).Error("bad mongo resp")
					return c.Status(500).JSON(&fiber.Map{
						"status":  500,
						"message": "Failed to read account.",
					})
				}
			} else {
				return c.Status(500).JSON(&fiber.Map{
					"status":  500,
					"message": "Failed to create or update the account (" + doc.Err().Error() + ")",
				})
			}
		} else {
			if err := doc.Decode(mongoUser); err != nil {
				return c.Status(500).JSON(&fiber.Map{
					"status":  500,
					"message": "Could not decode user document (" + err.Error() + ")",
				})
			}
		}

		var respError error
		// Check ban?
		if reason, err := redis.Client.HGet(c.Context(), "user:bans", mongoUser.ID.Hex()).Result(); err != redis.ErrNil {
			var ban *datastructure.Ban
			res := mongo.Collection(mongo.CollectionNameBans).FindOne(c.Context(), bson.M{"user_id": mongoUser.ID, "active": true})
			err = res.Err()
			if err == nil {
				_ = res.Decode(&ban)
				respError = fmt.Errorf(
					"You are currently banned for '%v'%v",
					reason,
					fmt.Sprintf(" until %v", utils.Ternary(ban != nil && !ban.ExpireAt.IsZero(), ban.ExpireAt.Format("Mon, 02 Jan 2006 15:04:05 MST"), "the universe fades out")),
				)
			} else {
				log.WithError(err).Error("mongo")
			}
		}

		authPl := &middleware.PayloadJWT{
			ID:           mongoUser.ID,
			TWID:         mongoUser.TwitchID,
			TokenVersion: mongoUser.TokenVersion,
			CreatedAt:    time.Now(),
		}

		authToken, err := jwt.Sign(authPl)
		if err != nil {
			log.WithError(err).Error("jwt")
			return c.Status(500).JSON(&fiber.Map{
				"status":  500,
				"message": "Failed to create user auth.",
			})
		}

		c.Cookie(&fiber.Cookie{
			Name:     "auth",
			Value:    authToken,
			HTTPOnly: false,
			Domain:   configure.Config.GetString("cookie_domain"),
			Secure:   configure.Config.GetBool("cookie_secure"),
			Expires:  time.Now().Add(time.Hour * 24 * 14),
		})

		return c.Redirect(configure.Config.GetString("website_url") + "/callback" + utils.Ternary(respError != nil, fmt.Sprintf("?error=%v", respError), "").(string))
	})

	return twitch
}
