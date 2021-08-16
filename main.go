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
	"github.com/SevenTV/ServerGo/src/redis"
	_ "github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/server"

	"github.com/SevenTV/ServerGo/src/server/api/tasks"
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

	// Get and cache roles*
	ctx := context.Background()
	roles, err := GetAllRoles(ctx)
	if err != nil {
		log.WithError(err).Error("could not get roles")
	}
	log.WithField("count", len(roles)).Infof("retrieved roles")

	// Sync bans
	bans, err := SyncBans(ctx)
	if err != nil {
		log.WithError(err).Error("could not sync bans")
	}
	log.WithField("count", len(bans)).Info("retrieved bans")

	go tasks.Start()

	select {}
}

func Cleanup() {
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
		return nil, err
	}

	// Set "AllRoles" value to mongo context
	cache.CachedRoles = roles
	return roles, nil
}

// SyncBans: Ensure active bans exist on the redis instance
func SyncBans(ctx context.Context) ([]*datastructure.Ban, error) {
	bans := []*datastructure.Ban{}
	cur, err := mongo.Collection(mongo.CollectionNameBans).Find(ctx, bson.M{
		"$or": bson.A{
			bson.M{"expire_at": nil},
			bson.M{"expire_at": bson.M{"$gt": time.Now()}},
		},
	})
	if err != nil {
		return nil, err
	}

	if err := cur.All(ctx, &bans); err != nil {
		return nil, err
	}

	// Sync with redis
	for _, b := range bans {
		if redis.Client.HExists(ctx, "user:bans", b.UserID.Hex()).Val() {
			continue
		}
		if err := redis.Client.HSet(ctx, "user:bans", b.UserID.Hex(), b.Reason).Err(); err != nil {
			log.WithError(err).Warn("SyncBans")
		}
	}

	return bans, nil
}

func panicHandler(output string) {
	fmt.Printf("PANIC OCCURED:\n\n%s\n", output)
	// Try to send a message to discord
	discord.SendPanic(output)

	os.Exit(1)
}
