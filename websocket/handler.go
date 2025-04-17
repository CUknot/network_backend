package websocket

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// Initialize the hub
func init() {
	InitHub()
}

// HandleConnection handles websocket connections
func HandleConnection(c *gin.Context) {
	// Get user ID from query parameter
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("error upgrading connection: %v", err)
		return
	}

	// Create a new client
	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: uint(userID),
		rooms:  make(map[uint]bool),
	}

	// Register client
	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}
