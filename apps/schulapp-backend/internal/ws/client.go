package ws

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// The frontend is served from a different origin/port during development
	// (Vite dev server) and from the same origin behind Caddy in production;
	// authentication happens via the JWT query param, not the Origin header.
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Client wraps one WebSocket connection with a buffered outbound queue so a
// slow reader can't block message broadcasting to other users.
type Client struct {
	conn *websocket.Conn
	out  chan []byte
}

// Serve upgrades the request and blocks until the connection closes,
// registering/unregistering itself with the hub around that lifetime.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID int) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &Client{conn: conn, out: make(chan []byte, 16)}
	h.register(userID, c)
	defer h.unregister(userID, c)

	go c.writePump()
	c.readPump()
}

func (c *Client) send(payload []byte) {
	select {
	case c.out <- payload:
	default:
		// Outbound queue full (client not reading fast enough) - drop rather
		// than block the broadcaster.
	}
}

func (c *Client) readPump() {
	defer c.conn.Close()
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		// Clients only receive; any inbound frame is just discarded, but we
		// must keep reading so control frames (ping/pong/close) get handled.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case payload, ok := <-c.out:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Log is a tiny seam kept so future connection-error metrics have a single
// place to hook into without touching call sites.
func Log(format string, args ...any) { log.Printf(format, args...) }
