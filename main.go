package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	_ "github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server"
	api_websocket "github.com/SevenTV/ServerGo/src/server/api/v2/websocket"
)

func init() {
	log.Infoln("Application Starting...")
}

func main() {
	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		log.Warnf("Invalid exit code specified in config (%v), using 0 as new exit code.", configCode)
		configCode = 0
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	s := server.New()

	go func() {
		sig := <-c
		log.Infof("sig=%v, gracefully shutting down...", sig)
		start := time.Now().UnixNano()

		// Run pre-shutdown cleanup
		Cleanup()

		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Shutdown(); err != nil {
				log.Errorf("failed to shutdown server, err=%v", err)
			}
		}()
		wg.Wait()

		log.Infof("Shutdown took, %.2fms", float64(time.Now().UnixNano()-start)/10e5)
		os.Exit(configCode)
	}()

	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Debugf("Alloc = %vM\tTotalAlloc = %vM\tSys = %vM\tNumGC = %v", m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024, m.NumGC)
			time.Sleep(5 * time.Second)
		}
	}()

	log.Infoln("Application Started.")

	// Get and cache roles
	roles, err := GetAllRoles(context.Background())
	if err != nil {
		log.Errorf("could not get roles, %s", err)
	}
	log.Infof("Retrieved %s roles", fmt.Sprint(len(roles)))

	select {}
}

func Cleanup() {
	// Remove websocket connections from Redis
	api_websocket.Cleanup()

	// Logout from discord
	_ = discord.Discord.CloseWithCode(1000)
}

// Get all roles available and cache into the mongo context
func GetAllRoles(ctx context.Context) ([]datastructure.Role, error) {
	roles := []datastructure.Role{}
	cur, err := mongo.Database.Collection("roles").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	roles = append(roles, *datastructure.DefaultRole) // Add default role
	if err := cur.All(ctx, &roles); err != nil {      // Fetch roles
		if err == mongo.ErrNoDocuments {
			return roles, nil
		}

		return nil, err
	}

	// Set "AllRoles" value to mongo context
	cache.CachedRoles = roles
	return roles, nil
}
