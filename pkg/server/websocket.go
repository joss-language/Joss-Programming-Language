package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// checkWebSocketOrigin verifies that incoming WebSocket connections originate from
// the same host or from explicitly allowed domains configured via APP_ALLOWED_ORIGINS or APP_URL.
func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser or same-origin direct clients without Origin header
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// 1. Same-origin comparison with request Host
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}

	// 2. Allow local loopback origins in local environments
	hostOnly := u.Hostname()
	if hostOnly == "localhost" || hostOnly == "127.0.0.1" || hostOnly == "::1" {
		reqHost := r.Host
		if colon := strings.Index(reqHost, ":"); colon != -1 {
			reqHost = reqHost[:colon]
		}
		if reqHost == "localhost" || reqHost == "127.0.0.1" || reqHost == "::1" {
			return true
		}
	}

	// 3. Configurable allowlist via APP_ALLOWED_ORIGINS (comma-separated origins or hosts)
	allowedList := os.Getenv("APP_ALLOWED_ORIGINS")
	if currentRuntime != nil && currentRuntime.Env != nil {
		if envAllowed, ok := currentRuntime.Env["APP_ALLOWED_ORIGINS"]; ok && envAllowed != "" {
			allowedList = envAllowed
		}
	}
	if allowedList != "" {
		for _, allowed := range strings.Split(allowedList, ",") {
			allowed = strings.TrimSpace(allowed)
			if allowed == "*" {
				return true
			}
			if strings.EqualFold(allowed, origin) || strings.EqualFold(allowed, u.Host) || strings.EqualFold(allowed, u.Hostname()) {
				return true
			}
		}
	}

	// 4. Check against APP_URL if configured
	appURL := os.Getenv("APP_URL")
	if currentRuntime != nil && currentRuntime.Env != nil {
		if val, ok := currentRuntime.Env["APP_URL"]; ok && val != "" {
			appURL = val
		}
	}
	if appURL != "" {
		if au, err := url.Parse(appURL); err == nil {
			if strings.EqualFold(au.Host, u.Host) {
				return true
			}
		}
	}

	return false
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWebSocketOrigin,
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			fmt.Println("[WebSocket] Nuevo cliente conectado")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				fmt.Println("[WebSocket] Cliente desconectado")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("error: %v", err)
			}
			break
		}
		fmt.Printf("[WebSocket] Recibido: %s\n", message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

// Global Hub Instance
var GlobalHub *Hub

func InitWebSocket() {
	GlobalHub = NewHub()
	go GlobalHub.Run()
}

func Broadcast(msg interface{}) {
	if GlobalHub != nil {
		b, _ := json.Marshal(msg)
		GlobalHub.broadcast <- b
	}
}
