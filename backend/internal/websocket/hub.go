package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

type Hub struct {
	clients    map[*Client]struct{}
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan Message, 512),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			data, err := json.Marshal(message)
			if err != nil {
				continue
			}
			stale := make([]*Client, 0)
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					stale = append(stale, client)
				}
			}
			h.mu.RUnlock()
			if len(stale) > 0 {
				h.mu.Lock()
				for _, client := range stale {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// sendToUser delivers data only to clients whose userID matches.
// Stale clients (full send buffer) are collected and cleaned up.
func (h *Hub) sendToUser(userID string, data []byte) {
	stale := make([]*Client, 0)
	h.mu.RLock()
	for client := range h.clients {
		if client.userID != userID {
			continue
		}
		select {
		case client.send <- data:
		default:
			stale = append(stale, client)
		}
	}
	h.mu.RUnlock()
	if len(stale) > 0 {
		h.mu.Lock()
		for _, client := range stale {
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		}
		h.mu.Unlock()
	}
}

// BroadcastDeploymentUpdate sends a status event only to the owning user's clients.
// If userID is empty the event is broadcast to every connected client (backwards-compat).
func (h *Hub) BroadcastDeploymentUpdate(userID, deploymentID, status string, details interface{}) {
	msg := Message{
		Type: "deployment_status",
		Payload: map[string]interface{}{
			"deployment_id": deploymentID,
			"status":        status,
			"details":       details,
		},
		Timestamp: time.Now().UTC(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if userID == "" {
		h.broadcast <- msg
		return
	}
	go h.sendToUser(userID, data)
}

// BroadcastLog sends a log event only to the owning user's clients.
// If userID is empty the event reaches every connected client.
func (h *Hub) BroadcastLog(userID, deploymentID, level, message string) {
	msg := Message{
		Type: "deployment_log",
		Payload: map[string]interface{}{
			"deployment_id": deploymentID,
			"level":         level,
			"message":       message,
		},
		Timestamp: time.Now().UTC(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if userID == "" {
		h.broadcast <- msg
		return
	}
	go h.sendToUser(userID, data)
}

