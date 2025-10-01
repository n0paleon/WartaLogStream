package main

import (
	"WartaLogStream/internal/adapters/storage"
	"WartaLogStream/internal/ports"
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"log"
	"sync/atomic"
	"time"
)

var (
	MaxLogEntries = 100
	PingTimeout   = 30 * time.Second
	PingInterval  = 5 * time.Second
	Port          = kingpin.Flag("port", "Port to listen on").Short('p').Default("3003").String()
)

type WSAction string

const (
	ActionPushLog      WSAction = "push_log"
	ActionSetNote      WSAction = "set_note"
	ActionPushFinished WSAction = "push_finished"
	ActionPing         WSAction = "ping"
)

type WSMessage struct {
	Action WSAction `json:"action"`
	Token  string   `json:"token,omitempty"`
	Data   string   `json:"data,omitempty"`
}

func main() {
	kingpin.Parse()
	sessionRegistry := ports.NewSessionRegistry()

	app := fiber.New(fiber.Config{
		Prefork:           false,
		CaseSensitive:     true,
		StrictRouting:     true,
		AppName:           "WartaLogStream",
		EnablePrintRoutes: true,
	})
	app.Static("/", "./public")

	// create session via HTTP
	app.Post("/session", func(ctx *fiber.Ctx) error {
		session := storage.NewInMemoryStorage(MaxLogEntries)
		sessionRegistry.Add(session)

		return ctx.JSON(fiber.Map{
			"session_id": session.GetID(),
			"token":      session.GetToken(),
		})
	})

	// WS writer
	app.Use("/session/:session_id/ws/writer",
		func(c *fiber.Ctx) error {
			token := c.Get("x-writer-token", "")
			if token == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"message": "token is required",
				})
			}

			sessionId := c.Params("session_id")
			session, ok := sessionRegistry.Get(sessionId)
			if !ok {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"message": "session not found",
				})
			}

			if err := session.ValidateToken(token); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"message": err.Error(),
				})
			}

			return c.Next()
		},
		websocket.New(func(c *websocket.Conn) {
			sessionId := c.Params("session_id")
			session, ok := sessionRegistry.Get(sessionId)
			if !ok {
				_ = c.WriteJSON(fiber.Map{"message": "invalid session id"})
				_ = c.Close()
				return
			}

			var lastPing int64 = time.Now().UnixNano()

			_ = c.SetReadDeadline(time.Now().Add(PingTimeout))
			c.SetPongHandler(func(string) error {
				_ = c.SetReadDeadline(time.Now().Add(PingTimeout))
				return nil
			})

			// goroutine cek ping timeout
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					deadline := time.Now().Add(PingInterval)
					session.UpdateWriterPing()
					if err := c.WriteControl(websocket.PingMessage, []byte{}, deadline); err != nil {
						log.Println("ping failed:", err)
						_ = session.Stop()
						_ = c.Close()
						return
					}
				}
			}()

			for {
				var msg WSMessage
				if err := c.ReadJSON(&msg); err != nil {
					return
				}
				atomic.StoreInt64(&lastPing, time.Now().UnixNano())

				switch msg.Action {
				case ActionPushLog:
					if err := session.PushLog(msg.Data); err != nil {
						_ = c.WriteJSON(fiber.Map{"error": err.Error()})
					}
				case ActionSetNote:
					if err := session.SetNote(msg.Data); err != nil {
						_ = c.WriteJSON(fiber.Map{"error": err.Error()})
					}
				case ActionPushFinished:
					err := session.Stop()
					_ = c.WriteJSON(fiber.Map{"message": "session finished", "error": err})
					_ = c.Close()
					return
				case ActionPing:
					session.UpdateWriterPing()
				default:
					_ = c.WriteJSON(fiber.Map{"message": "invalid action"})
				}
			}
		}))

	// WS subscriber
	app.Use("/session/:session_id/ws/subscriber", websocket.New(func(c *websocket.Conn) {
		sessionId := c.Params("session_id")
		session, ok := sessionRegistry.Get(sessionId)
		if !ok {
			_ = c.WriteJSON(fiber.Map{"message": "invalid session id"})
			_ = c.Close()
			return
		}

		eventCh := session.SubscribeLog()
		defer session.UnsubscribeLog(eventCh)
		var lastPing int64 = time.Now().UnixNano()

		_ = c.SetReadDeadline(time.Now().Add(PingTimeout))
		c.SetPongHandler(func(string) error {
			_ = c.SetReadDeadline(time.Now().Add(PingTimeout))
			return nil
		})

		// goroutine cek ping timeout
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				deadline := time.Now().Add(PingInterval)
				if err := c.WriteControl(websocket.PingMessage, []byte{}, deadline); err != nil {
					log.Println("ping failed:", err)
					_ = c.Close()
					return
				}
			}
		}()

		// goroutine untuk broadcast events
		go func() {
			for evt := range eventCh {
				if err := c.WriteJSON(evt); err != nil {
					_ = c.Close()
					return
				}
				if evt.Type == ports.EventStatus && evt.Data == "Stopped" {
					_ = c.Close()
					return
				}
			}
		}()

		for {
			var msg WSMessage
			if err := c.ReadJSON(&msg); err != nil {
				session.UnsubscribeLog(eventCh)
				return
			}
			atomic.StoreInt64(&lastPing, time.Now().UnixNano())
		}
	}))

	log.Fatal(app.Listen(fmt.Sprintf(":%s", *Port)))
}
