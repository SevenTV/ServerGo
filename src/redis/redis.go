package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
	"github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
)

var (
	errInvalidResp = fmt.Errorf("invalid resp from redis")
)

func init() {
	options, err := redis.ParseURL(configure.Config.GetString("redis_uri"))
	options.DB = configure.Config.GetInt("redis_db")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}

	Client = redis.NewClient(options)

	sub = Client.Subscribe(context.Background())
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.WithField("err", err).Fatal("panic in subs")
			}
		}()
		ch := sub.Channel()
		var (
			msg     *redis.Message
			payload []byte
		)
		for {
			msg = <-ch
			payload = []byte(msg.Payload) // dont change we want to copy the memory due to concurrency.
			subsMtx.Lock()
			for _, s := range subs[msg.Channel] {
				go func(s *redisSub) {
					defer func() {
						if err := recover(); err != nil {
							log.WithField("err", err).Error("panic in subs")
						}
					}()
					s.ch <- payload
				}(s)
			}
			subsMtx.Unlock()
		}
	}()

	if err = ReloadScripts(); err != nil {
		log.WithError(err).Fatal("redis failed")
	}
}

func ReloadScripts() error {
	box := packr.New("lua", "./lua")

	tokenConsumerLuaScript, err := box.FindString("token-consumer.lua")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*25)
	defer cancel()
	v, err := Client.ScriptLoad(ctx, tokenConsumerLuaScript).Result()
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	tokenConsumerLuaScriptSHA1 = v

	getCacheLuaScript, err := box.FindString("get-cache.lua")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	v, err = Client.ScriptLoad(ctx, getCacheLuaScript).Result()
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	getCacheLuaScriptSHA1 = v

	setCacheLuaScript, err := box.FindString("set-cache.lua")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	v, err = Client.ScriptLoad(ctx, setCacheLuaScript).Result()
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	setCacheLuaScriptSHA1 = v

	invalidateCacheLuaScript, err := box.FindString("invalidate-cache.lua")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	v, err = Client.ScriptLoad(ctx, invalidateCacheLuaScript).Result()
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	invalidateCacheLuaScriptSHA1 = v

	invalidateCommonIndexCacheLuaScript, err := box.FindString("invalidate-common-index-cache.lua")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	v, err = Client.ScriptLoad(ctx, invalidateCommonIndexCacheLuaScript).Result()
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	invalidateCommonIndexCacheLuaScriptSHA1 = v

	rateLimitLuaScript, err := box.FindString("rate-limit.lua")
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	v, err = Client.ScriptLoad(ctx, rateLimitLuaScript).Result()
	if err != nil {
		log.WithError(err).Fatal("redis failed")
	}
	RateLimitScriptSHA1 = v

	return nil
}

var Client *redis.Client

var (
	sub     *redis.PubSub
	subs    = map[string][]*redisSub{}
	subsMtx = sync.Mutex{}
)

type redisSub struct {
	ch chan []byte
}

var lockerClient *redislock.Client

func GetLocker() *redislock.Client {
	if lockerClient == nil {
		lockerClient = redislock.New(Client)
	}

	return lockerClient
}

type Message = redis.Message

type StringCmd = redis.StringCmd

type StringStringMapCmd = redis.StringStringMapCmd

type PubSub = redis.PubSub

type Z = redis.Z

const ErrNil = redis.Nil

var RateLimitScriptSHA1 string
