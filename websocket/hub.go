package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Rooms mapping (roomID -> clients)
	rooms map[uint]map[*Client]bool

	// Mutex for rooms map
	roomsMux sync.RWMutex

	// Inbound messages from the clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client
}

// NewHub creates a new hub instance
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[uint]map[*Client]bool),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)

				// Remove client from all rooms
				h.roomsMux.Lock()
				for roomID, clients := range h.rooms {
					if _, ok := clients[client]; ok {
						delete(h.rooms[roomID], client)
						// Clean up empty rooms
						if len(h.rooms[roomID]) == 0 {
							delete(h.rooms, roomID)
						}
					}
				}
				h.roomsMux.Unlock()
			}
		case message := <-h.broadcast:
			// Parse the message to determine which room to broadcast to
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("error unmarshaling broadcast message: %v", err)
				continue
			}

			// Handle message based on type
			if msg.Type == "message" {
				// Extract room ID from payload
				var payload struct {
					RoomID uint `json:"room_id"`
				}

				payloadBytes, err := json.Marshal(msg.Payload)
				if err != nil {
					log.Printf("error marshaling payload: %v", err)
					continue
				}

				if err := json.Unmarshal(payloadBytes, &payload); err != nil {
					log.Printf("error unmarshaling payload: %v", err)
					continue
				}

				// Broadcast to specific room
				h.broadcastToRoom(payload.RoomID, message)
			}
		}
	}
}

// joinRoom adds a client to a room
func (h *Hub) joinRoom(client *Client, roomID uint) {
	h.roomsMux.Lock()
	defer h.roomsMux.Unlock()

	if _, ok := h.rooms[roomID]; !ok {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	h.rooms[roomID][client] = true
}

// leaveRoom removes a client from a room
func (h *Hub) leaveRoom(client *Client, roomID uint) {
	h.roomsMux.Lock()
	defer h.roomsMux.Unlock()

	if _, ok := h.rooms[roomID]; ok {
		delete(h.rooms[roomID], client)
		// Clean up empty rooms
		if len(h.rooms[roomID]) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

// broadcastToRoom sends a message to all clients in a room
func (h *Hub) broadcastToRoom(roomID uint, message []byte) {
	h.roomsMux.RLock()
	defer h.roomsMux.RUnlock()

	if clients, ok := h.rooms[roomID]; ok {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(clients, client)
				delete(h.clients, client)
			}
		}
	}
}

// BroadcastToRoom sends a message to all clients in a room
func BroadcastToRoom(roomID uint, msgType string, payload interface{}) {
	msg := Message{
		Type:    msgType,
		Payload: payload,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshaling message: %v", err)
		return
	}

	hub.broadcastToRoom(roomID, msgBytes)
}

// Global hub instance
var hub *Hub

// InitHub initializes the global hub
func InitHub() {
	hub = NewHub()
	go hub.Run()
}
