package api_websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/redis"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const heartbeatInterval int32 = 90 // Heartbeat interval in seconds

var (
	connections = make(map[uuid.UUID]*Conn)
	mtx         = sync.Mutex{}
	wg          = sync.WaitGroup{}
)

type Stat struct {
	UUID          uuid.UUID               `json:"id"`     // The connection's unique ID
	Sequence      int32                   `json:"seq"`    // The amount of events sent by the server to this connection
	CreatedAt     time.Time               `json:"age"`    // The time at which this connection became active
	Subscriptions []WebSocketSubscription `json:"subs"`   // A list of active subscription types&
	Closed        bool                    `json:"closed"` // True if the connection has been closed
	IP            string                  `json:"ip"`     // The client's IP Address
	Lock          *sync.Mutex
	RedisKey      string
}

func Cleanup() {
	mtx.Lock()
	log.Infof("<WebSocket> Closing %d connections", len(connections))
	for _, conn := range connections {
		_ = conn.Close()
	}
	mtx.Unlock()
	wg.Wait()
}

func WebSocket(app fiber.Router) {
	ws := app.Group("/ws")

	ws.Use("/", func(c *fiber.Ctx) error {
		if !configure.Config.GetBool("websocket.enabled") {
			return c.Status(503).SendString("WebSocket is currently disabled")
		}

		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			var ip string
			if len(c.IPs()) > 0 {
				ip = c.IPs()[0]
			} else {
				ip = c.Context().RemoteIP().String()
			}

			c.Locals("ClientIP", ip)
			return c.Next()
		}
		// Upgrade Required
		return c.SendStatus(426)
	})

	// WebSocket Endpoint:
	// Subscribe to event channels
	ws.Get("/", websocket.New(func(conn *websocket.Conn) {
		defer func() {
			if err := recover(); err != nil {
				log.WithField("err", err).Error("panic in websocket")
			}
		}()
		c := transform(conn)
		c.SendOpGreet() // Send an hello payload to the user

		// This socket has connected
		log.Infof("<WS> Connect: %v", c.RemoteAddr().String())
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), WebSocketConnKey, c))
		c.cancel = cancel
		mtx.Lock()
		connections[c.Stat.UUID] = c
		mtx.Unlock()

		c.SetCloseHandler(func(code int, text string) error {
			cancel()
			return nil
		})

		wg.Add(1)

		// Event channels
		// chIdentified := make(chan bool)

		defer func() {
			// Cancel the context so everything closes up
			defer wg.Done()
			log.Infof("<WS> Disconnect: %v", c.RemoteAddr().String())

			// Handle connection removal
			c.Stat.Closed = true // Set closed stat to true
			mtx.Lock()
			delete(connections, c.Stat.UUID) // Remove from connections map
			mtx.Unlock()
		}()

		// Wait for the client to send their first heartbeat
		// Failure to do so in time will disconnect the socket
		heartbeat := awaitHeartbeat(ctx, c, time.Duration(heartbeatInterval)*time.Second)

		var (
			b   []byte
			err error
			msg WebSocketMessageInbound
		)

		go func() {
			defer cancel()
			for { // Listen to client messages
				if _, b, err = c.ReadMessage(); err != nil {
					return
				}
				// Handle invalid payload
				if err = json.Unmarshal(b, &msg); err != nil {
					c.SendClosure(websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
					return
				}

				switch msg.Op {
				// Opcode: HEARTBEAT (Client signals the server that the connection is alive)
				case WebSocketMessageOpHeartbeat:
					heartbeat()

				// Opcode: IDENTIFY (Client wants to sign in to make authorized commands)
				case WebSocketMessageOpIdentify:
					// TODO
					// chIdentified <- true

				// Opcode: SUBSCRIBE (Client wants to start receiving events from a specified source)
				case WebSocketMessageOpSubscribe:
					var data WebSocketSubscription
					c.decodeMessageData(ctx, msg, &data) // Decode message data

					subscription := data.Type // The subscription that the client wants to create

					switch subscription {
					case WebSocketSubscriptionChannelEmotes: // Subscribe: CHANNEL EMOTES
						go createChannelEmoteSubscription(ctx, c, data)

					default: // Unknown Subscription
						c.SendClosure(1003, "Unknown Subscription Type")
					}

				default:
					c.SendClosure(1003, "Invalid Opcode")
				}
			}
		}()

		<-ctx.Done()
	}))
}

type Conn struct {
	*websocket.Conn
	helpers webSocketHelpers
	Stat    Stat
	cancel  context.CancelFunc
}

func (c *Conn) Close() error {
	c.cancel()
	return c.Conn.Close()
}

func transform(ws *websocket.Conn) *Conn {
	id := uuid.New()
	return &Conn{
		Conn: ws,
		helpers: webSocketHelpers{
			subscriberCallersUserEmotes: make(map[string]*eventCallback),
		},
		Stat: Stat{
			UUID:          id,
			Subscriptions: []WebSocketSubscription{},
			CreatedAt:     time.Now(),
			Lock:          &sync.Mutex{},
			RedisKey:      fmt.Sprintf("ws:connections:%v", id.String()),
			IP:            ws.Locals("ClientIP").(string),
		},
	}
}

func (c *Conn) SendOpDispatch(ctx context.Context, data interface{}, t string) {
	// Increase sequence
	c.Stat.Sequence++
	_ = redis.Client.HIncrBy(ctx, c.Stat.RedisKey, "seq", 1)

	c.sendMessage(&WebSocketMessageOutbound{
		Op:       WebSocketMessageOpDispatch,
		Data:     data,
		Sequence: &c.Stat.Sequence,
		Type:     &t,
	})
}

func (c *Conn) SendOpGreet() {
	c.sendMessage(&WebSocketMessageOutbound{
		Op: WebSocketMessageOpHello,
		Data: WebSocketMessageDataHello{
			Timestamp:         time.Now(),
			HeartbeatInterval: heartbeatInterval * 1000,
		},
	})
}

func (c *Conn) SendOpHeartbeatAck() {
	c.sendMessage(&WebSocketMessageOutbound{
		Op:   WebSocketMessageOpHeartbeatAck,
		Data: struct{}{},
	})
}

func (c *Conn) SendClosure(code int, message string) {
	if c == nil || c.Stat.Closed {
		return
	}
	c.Stat.Lock.Lock()
	c.Stat.Closed = true

	b := websocket.FormatCloseMessage(code, message)

	if err := c.WriteMessage(websocket.CloseMessage, b); err != nil {
		log.WithError(err).Error("websocket failed to write closure message")
	}
	c.Close()
	c.Stat.Lock.Unlock()
}

func (c *Conn) sendMessage(message *WebSocketMessageOutbound) {
	if c.Stat.Closed {
		return
	}
	c.Stat.Lock.Lock()

	if err := c.WriteJSON(message); err != nil {
		log.WithError(err).Error("websocket failed to write json message")
	}
	c.Stat.Lock.Unlock()
}

func (c *Conn) decodeMessageData(ctx context.Context, msg WebSocketMessageInbound, v interface{}) {
	if err := json.Unmarshal(msg.Data, &v); err != nil {
		c.SendClosure(websocket.CloseInvalidFramePayloadData, fmt.Sprintf("Invalid JSON: %v", err))
	}
}

type WebSocketMessageOutbound struct {
	Op       int8        `json:"op"` // The message operation code
	Data     interface{} `json:"d"`
	Sequence *int32      `json:"seq"`
	Type     *string     `json:"t"`
}

type WebSocketMessageInbound struct {
	Op   int8            `json:"op"`
	Data json.RawMessage `json:"d"`
}

type WebSocketMessageDataHello struct {
	Timestamp         time.Time `json:"timestamp"`          // The timestamp at which HELLO was sent
	HeartbeatInterval int32     `json:"heartbeat_interval"` // How often the user is expected to send heartbeats in milliseconds
}

type WebSocketMessageDataServerClosure struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type WebSocketSubscription struct {
	Type   int8              `json:"type"`
	Params map[string]string `json:"params"`
}

const (
	WebSocketMessageOpDispatch int8 = iota
	WebSocketMessageOpHello
	WebSocketMessageOpHeartbeat
	WebSocketMessageOpHeartbeatAck
	WebSocketMessageOpIdentify
	WebSocketMessageOpServerClosure
	WebSocketMessageOpSubscribe
)

const (
	WebSocketSubscriptionChannelEmotes int8 = 1 + iota
)

const WebSocketConnKey = utils.Key("conn")
const WebSocketSeqKey = utils.Key("seq")
const WebSocketStatKey = utils.Key("stat")
