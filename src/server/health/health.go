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
	downedServices := map[string]bool{
		"redis": false,
		"mongo": false,
	}

	mtx := sync.Mutex{}
	app.Get("/health", func(c *fiber.Ctx) error {
		mtx.Lock()
		defer mtx.Unlock()

		isDown := false

		redisCtx, cancel := context.WithTimeout(c.Context(), time.Second*10)
		defer cancel()
		// CHECK REDIS
		if ping := redis.Client.Ping(redisCtx).Val(); ping == "" {
			log.Error("health, REDIS IS DOWN")
			isDown = true
			if down := downedServices["redis"]; !down {
				go discord.SendServiceDown("redis")
				downedServices["redis"] = true
			}
		} else {
			if down := downedServices["redis"]; down {
				go discord.SendServiceRestored("redis")
				downedServices["redis"] = false
			}
		}

		// CHECK MONGO
		mongoCtx, cancel := context.WithTimeout(c.Context(), time.Second*10)
		defer cancel()
		if err := mongo.Database.Client().Ping(mongoCtx, nil); err != nil {
			log.Error("health, MONGO IS DOWN")
			isDown = true
			if down := downedServices["mongo"]; !down {
				go discord.SendServiceDown("mongo")
				downedServices["mongo"] = true
			}
		} else {
			if down := downedServices["mongo"]; down {
				go discord.SendServiceRestored("mongo")
				downedServices["mongo"] = false
			}
		}

		if isDown {
			return c.SendStatus(503)
		}

		return c.Status(200).SendString("OK")
	})

}
