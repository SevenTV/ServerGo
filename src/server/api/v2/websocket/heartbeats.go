package api_websocket

import (
	"context"
	"time"

	"github.com/gofiber/websocket/v2"
	log "github.com/sirupsen/logrus"
)

func awaitHeartbeat(ctx context.Context, c *Conn, duration time.Duration) func() {
	ticker := time.NewTicker(duration + time.Second*30)
	lastTrigger := time.Time{}

	trigger := func() {
		if ctx.Err() != nil {
			return
		}
		c.SendOpHeartbeatAck()
		lastTrigger = time.Now()
	}

	go func() {
		defer ticker.Stop()
		defer func() {
			if err := recover(); err != nil {
				log.WithField("err", err).Error("panic")
			}
		}()
		// Wait for the user to send a heartbeat, or the socket will timeout
		for {
			select {
			case <-ctx.Done(): // Connection ends
				return
			case <-time.After(duration + time.Second*30):
				// Client does not send heartbeat: timeout
				if time.Since(lastTrigger) > duration+time.Second*30 {
					c.SendClosure(websocket.ClosePolicyViolation, "Client failed to send heartbeat")
					return
				}
			}
		}
	}()

	return trigger
}
