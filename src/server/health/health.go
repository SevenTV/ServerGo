package health

import (
	"context"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/gofiber/fiber/v2"

	log "github.com/sirupsen/logrus"
)

func Health(app fiber.Router) {
	downedServices := sync.Map{}
	downedServices.Store("redis", false)
	downedServices.Store("mongo", false)

	app.Get("/health", func(c *fiber.Ctx) error {
		down := false

		redisCtx, cancel := context.WithTimeout(c.Context(), time.Second*10)
		defer cancel()
		// CHECK REDIS
		if ping := redis.Client.Ping(redisCtx).Val(); ping == "" {
			log.Error("health, REDIS IS DOWN")
			down = true
			if down, _ := downedServices.Load("redis"); !down.(bool) {
				go discord.SendServiceDown("redis")
				downedServices.Store("redis", true)
			}
		} else {
			if down, _ := downedServices.Load("redis"); down.(bool) {
				go discord.SendServiceRestored("redis")
				downedServices.Store("redis", false)
			}
		}

		// CHECK MONGO
		mongoCtx, cancel := context.WithTimeout(c.Context(), time.Second*10)
		defer cancel()
		if err := mongo.Database.Client().Ping(mongoCtx, nil); err != nil {
			log.Error("health, MONGO IS DOWN")
			down = true
			if down, _ := downedServices.Load("mongo"); !down.(bool) {
				go discord.SendServiceDown("mongo")
				downedServices.Store("mongo", true)
			}
		} else {
			if down, _ := downedServices.Load("redis"); down.(bool) {
				go discord.SendServiceRestored("mongo")
				downedServices.Store("mongo", false)
			}
		}

		if down {
			return c.SendStatus(503)
		}

		return c.Status(200).SendString("OK")
	})

}
