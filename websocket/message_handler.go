package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/models"
)

// MessagePayload represents the structure of a message payload
type MessagePayload struct {
	RoomID  uint   `json:"room_id"`
	Content string `json:"content"`
}

// SaveMessageToDB saves a message to the database and returns the saved message
func SaveMessageToDB(userID uint, payload MessagePayload) (models.Message, error) {
	// Create message
	message := models.Message{
		Content: payload.Content,
		RoomID:  payload.RoomID,
		UserID:  userID,
	}

	// Save to database
	if err := database.DB.Create(&message).Error; err != nil {
		return message, err
	}

	// Load user data for the message
	if err := database.DB.Preload("User").First(&message, message.ID).Error; err != nil {
		log.Printf("Error loading user data for message: %v", err)
	}

	return message, nil
}

// HandleIncomingMessage processes an incoming WebSocket message
func HandleIncomingMessage(client *Client, messageBytes []byte) {
	var msg Message
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	switch msg.Type {
	case "join_room":
		if roomID, ok := msg.Payload.(string); ok {
			roomIDUint := parseRoomID(roomID)
			client.joinRoom(roomIDUint)

			// Update last read timestamp for this room
			updateLastReadTime(client.userID, roomIDUint)
		}
	case "leave_room":
		if roomID, ok := msg.Payload.(string); ok {
			roomIDUint := parseRoomID(roomID)
			client.leaveRoom(roomIDUint)
		}
	case "message":
		// Extract message payload
		payloadBytes, err := json.Marshal(msg.Payload)
		if err != nil {
			log.Printf("Error marshaling payload: %v", err)
			return
		}

		var payload MessagePayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			log.Printf("Error unmarshaling message payload: %v", err)
			return
		}

		// Check if user is in the room
		if !client.inRoom(payload.RoomID) {
			log.Printf("User %d attempted to send message to room %d without joining",
				client.userID, payload.RoomID)
			return
		}

		// Save message to database
		savedMessage, err := SaveMessageToDB(client.userID, payload)
		if err != nil {
			log.Printf("Error saving message to database: %v", err)
			return
		}

		// Broadcast the saved message to the room
		responseMsg := Message{
			Type:    "message",
			Payload: savedMessage,
		}

		responseBytes, err := json.Marshal(responseMsg)
		if err != nil {
			log.Printf("Error marshaling response message: %v", err)
			return
		}

		client.hub.broadcastToRoom(payload.RoomID, responseBytes)
	case "invite_users":
		// Extract invite payload
		payloadBytes, err := json.Marshal(msg.Payload)
		if err != nil {
			log.Printf("Error marshaling invite payload: %v", err)
			return
		}

		var payload InvitePayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			log.Printf("Error unmarshaling invite payload: %v", err)
			return
		}

		// Handle the invitation
		HandleInviteUsers(client, payload)
	case "accept_invite":
		// Extract room ID from payload
		if roomID, ok := msg.Payload.(string); ok {
			HandleAcceptInvite(client, roomID)
		}
	case "reject_invite":
		// Extract room ID from payload
		if roomID, ok := msg.Payload.(string); ok {
			HandleRejectInvite(client, roomID)
		}
	}
}

// updateLastReadTime updates the last read timestamp for a user in a room
func updateLastReadTime(userID, roomID uint) {
	var roomUser models.RoomUser
	result := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).First(&roomUser)

	if result.Error != nil {
		log.Printf("Error finding room user: %v", result.Error)
		return
	}

	// Update last read time
	roomUser.LastReadAt = time.Now()
	if err := database.DB.Save(&roomUser).Error; err != nil {
		log.Printf("Error updating last read time: %v", err)
	}
}
