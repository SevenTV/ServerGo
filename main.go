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

	"github.com/mitchellh/panicwrap"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/cache"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	_ "github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server"

	"github.com/SevenTV/ServerGo/src/server/api/tasks"
	api_websocket "github.com/SevenTV/ServerGo/src/server/api/v2/websocket"
)

func init() {
	log.Infoln("starting")
}

func main() {
	// Catch panics - send alert to discord channel optionally
	exitStatus, err := panicwrap.BasicWrap(panicHandler)
	if err != nil {
		// PANIC-CEPTION. BRRRRRRRRRRRRRRRRRRRRRRRR
		panic(err)
	}
	if exitStatus >= 0 {
		os.Exit(exitStatus)
	}

	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		log.WithField("requested_exit_code", configCode).Warn("invalid exit code specified in config using 0 as new exit code")
		configCode = 0
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	s := server.New()

	go func() {
		sig := <-c
		log.WithField("sig", sig).Info("stop issued")

		start := time.Now().UnixNano()

		// Run pre-shutdown cleanup
		Cleanup()

		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Shutdown(); err != nil {
				log.WithError(err).Error("failed to shutdown server")
			}
		}()
		wg.Wait()

		log.WithField("duration", float64(time.Now().UnixNano()-start)/10e5).Infof("shutdown")
		os.Exit(configCode)
	}()

	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.WithField("alloc", m.Alloc).WithField("total_alloc", m.TotalAlloc).WithField("sys", m.Sys).WithField("numgc", m.NumGC).Debug("stats")
			time.Sleep(5 * time.Second)
		}
	}()

	log.Infoln("started")

	// Get and cache roles
	roles, err := GetAllRoles(context.Background())
	if err != nil {
		log.WithError(err).Error("could not get roles")
	}
	log.WithField("count", len(roles)).Infof("retrieved roles")

	go tasks.Start()

	if true {
		panic("xd")
	}
	select {}
}

func Cleanup() {
	// Remove websocket connections from Redis
	api_websocket.Cleanup()

	// Cleanup ongoing tasks
	tasks.Cleanup()

	// Logout from discord
	_ = discord.Discord.CloseWithCode(1000)
}

// Get all roles available and cache into the mongo context
func GetAllRoles(ctx context.Context) ([]datastructure.Role, error) {
	roles := []datastructure.Role{}
	cur, err := mongo.Collection(mongo.CollectionNameRoles).Find(ctx, bson.M{})
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

func panicHandler(output string) {
	fmt.Printf("PANIC OCCURED:\n\n%s\n", output)
	// Try to send a message to discord
	discord.SendPanic(output)

	os.Exit(1)
}
