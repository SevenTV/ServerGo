package v2

import (
	"github.com/SevenTV/ServerGo/src/server/api/v2/chatterino"
	"github.com/SevenTV/ServerGo/src/server/api/v2/gql"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest"
	api_websocket "github.com/SevenTV/ServerGo/src/server/api/v2/websocket"
	"github.com/gofiber/fiber/v2"
)

func API(app fiber.Router) fiber.Router {
	api := app.Group("/v2")

	api_websocket.WebSocket(api)
	Twitch(api)
	rest.RestV2(api)
	gql.GQL(api)
	chatterino.Chatterino(api)

	return api
}
