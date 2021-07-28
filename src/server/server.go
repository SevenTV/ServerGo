package server

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/jwt"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/redis"
	apiv2 "github.com/SevenTV/ServerGo/src/server/api/v2"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/gofiber/fiber/v2"
)

type Server struct {
	app      *fiber.App
	listener net.Listener
}

func New() *Server {
	l, err := net.Listen(configure.Config.GetString("conn_type"), configure.Config.GetString("conn_uri"))
	if err != nil {
		log.Fatalf("failed to start listner for http server, err=%v", err)
		return nil
	}

	server := &Server{
		app: fiber.New(fiber.Config{
			BodyLimit:                    2e16,
			StreamRequestBody:            true,
			DisablePreParseMultipartForm: true,
		}),
		listener: l,
	}

	server.app.Use(middleware.Logger())

	downedServices := map[string]bool{
		"redis": false,
		"mongo": false,
	}
	server.app.Get("/health", func(c *fiber.Ctx) error {
		ctx := context.Background()
		// CHECK REDIS
		if ping := redis.Client.Ping(ctx).Val(); ping == "" {
			log.Errorf("health, REDIS IS DOWN")

			if down := downedServices["redis"]; !down {
				go discord.SendServiceDown("redis")
			}
			downedServices["redis"] = true
			return c.SendStatus(503)
		} else {
			if down := downedServices["redis"]; down {
				go discord.SendServiceRestored("redis")
			}
			downedServices["redis"] = false
		}

		// CHECK MONGO
		ctx, cancel := context.WithCancel(ctx)
		pong := false
		go func() { // Initiate a ping to mongo
			err := mongo.Database.Client().Ping(ctx, nil)
			if err == nil { // No error: OK, service is healthy
				pong = true
			}
			cancel() // Cancel the context
		}()

		// Create a timeout
		// If mongo fails to respond with a pong in time, we must end the rquest with 503 Service Unavailable
		timer := time.NewTimer(3 * time.Second)
		for {
			select {
			case <-ctx.Done():
				break
			case <-timer.C:
				cancel()
				break
			}
			break
		}
		timer.Stop()
		if !pong {
			log.Errorf("health, MONGO IS DOWN")

			if down := downedServices["mongo"]; !down {
				go discord.SendServiceDown("mongo")
			}
			downedServices["mongo"] = true
			return c.SendStatus(503)
		} else {
			if down := downedServices["mongo"]; down {
				go discord.SendServiceRestored("mongo")
			}
			downedServices["mongo"] = false
		}

		<-ctx.Done()
		return c.Status(200).SendString("OK")
	})

	server.app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Node-Name", configure.NodeName)
		c.Set("X-Pod-Name", configure.PodName)
		c.Set("X-Pod-Internal-Address", configure.PodIP)

		delete := true
		auth := c.Cookies("auth")
		if auth != "" {
			splits := strings.Split(auth, ".")
			if len(splits) != 3 {
				pl := &middleware.PayloadJWT{}
				if err := jwt.Verify(splits, pl); err == nil {
					if pl.CreatedAt.After(time.Now().Add(-time.Hour * 24 * 60)) {
						delete = false
						c.Cookie(&fiber.Cookie{
							Name:     "auth",
							Value:    auth,
							Domain:   configure.Config.GetString("cookie_domain"),
							Expires:  time.Now().Add(time.Hour * 24 * 14),
							Secure:   configure.Config.GetBool("cookie_secure"),
							HTTPOnly: false,
						})
					}
				}
			}
			if delete {
				c.Cookie(&fiber.Cookie{
					Name:     "auth",
					Domain:   configure.Config.GetString("cookie_domain"),
					MaxAge:   -1,
					Secure:   configure.Config.GetBool("cookie_secure"),
					HTTPOnly: false,
				})
			}
		}

		return c.Next()
	})

	apiv2.API(server.app)

	server.app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).JSON(&fiber.Map{
			"status":  404,
			"message": "Not Found",
		})
	})

	go func() {
		err = server.app.Listener(server.listener)
		if err != nil {
			log.WithError(err).Fatal("failed to start http server")
		}
	}()

	return server
}

func (s *Server) Shutdown() error {
	return s.listener.Close()
}
