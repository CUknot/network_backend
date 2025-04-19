package websocket

import (
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 10000
)

// Client represents a connected websocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   uint
	rooms    map[uint]bool
	roomsMux sync.RWMutex
}

// Message represents a websocket message
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		// Use the message handler to process the message
		HandleIncomingMessage(c, message)
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// joinRoom adds the client to a room
func (c *Client) joinRoom(roomID uint) {
	c.roomsMux.Lock()
	defer c.roomsMux.Unlock()
	c.rooms[roomID] = true
	c.hub.joinRoom(c, roomID)
}

// leaveRoom removes the client from a room
func (c *Client) leaveRoom(roomID uint) {
	c.roomsMux.Lock()
	defer c.roomsMux.Unlock()
	delete(c.rooms, roomID)
	c.hub.leaveRoom(c, roomID)
}

// inRoom checks if the client is in a specific room
func (c *Client) inRoom(roomID uint) bool {
	c.roomsMux.RLock()
	defer c.roomsMux.RUnlock()
	return c.rooms[roomID]
}

// parseRoomID converts a string room ID to uint
func parseRoomID(roomID string) uint {
	id, err := strconv.ParseUint(roomID, 10, 64)
	if err != nil {
		log.Printf("error parsing room ID: %v", err)
		return 0
	}
	return uint(id)
}
