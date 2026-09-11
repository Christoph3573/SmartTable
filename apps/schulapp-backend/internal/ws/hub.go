// Package ws holds a minimal per-user WebSocket registry used to push chat
// messages and unread-count updates to connected browser tabs in real time.
package ws

import "sync"

// Hub tracks live connections per user. A user may have several open tabs,
// so each user id maps to a set of connections.
type Hub struct {
	mu      sync.RWMutex
	clients map[int]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[int]map[*Client]struct{})}
}

func (h *Hub) register(userID int, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*Client]struct{})
	}
	h.clients[userID][c] = struct{}{}
}

func (h *Hub) unregister(userID int, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[userID], c)
	if len(h.clients[userID]) == 0 {
		delete(h.clients, userID)
	}
}

// SendToUser delivers payload to every open connection of that user, if any.
func (h *Hub) SendToUser(userID int, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		c.send(payload)
	}
}

// SendToUsers is a convenience wrapper for broadcasting to several recipients.
func (h *Hub) SendToUsers(userIDs []int, payload []byte) {
	for _, id := range userIDs {
		h.SendToUser(id, payload)
	}
}
