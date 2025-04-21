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
			log.Printf("Invite accepted")
		}
	case "reject_invite":
		// Extract room ID from payload
		if roomID, ok := msg.Payload.(string); ok {
			HandleRejectInvite(client, roomID)
			log.Printf("Invite rejected")
		}
	case "status":
		if status, ok := msg.Payload.(string); ok {
			HandleStatus(client, status)
		}
	case "leave_group":
		if roomID, ok := msg.Payload.(string); ok {
			roomIDUint := parseRoomID(roomID)
			client.leaveGroup(roomIDUint)
			HandleLeaveGroup(client, roomID)
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

func HandleStatus(client *Client, status string) {
	log.Printf("User %d updating status to: %s", client.userID, status)

	client.statusMu.Lock()
	client.status = status
	client.statusMu.Unlock()

	client.hub.roomsMux.RLock()
	defer client.hub.roomsMux.RUnlock()

	for roomID, clients := range client.hub.rooms {
		if _, ok := clients[client]; ok {
			log.Printf("User %d is in room %d, broadcasting status update", client.userID, roomID)

			statusMsg := Message{
				Type: "status_update",
				Payload: map[string]interface{}{
					"user_id": client.userID,
					"status":  status,
					"room_id": roomID,
				},
			}

			msgBytes, err := json.Marshal(statusMsg)
			if err != nil {
				log.Printf("Error marshaling status update: %v", err)
				continue
			}

			for roomClient := range clients {
				if roomClient != client {
					select {
					case roomClient.send <- msgBytes:
						log.Printf("Sent status update of user %d to user %d", client.userID, roomClient.userID)
					default:
						log.Printf("Client %d send channel is full or closed. Removing from hub.", roomClient.userID)
						close(roomClient.send)
						delete(client.hub.clients, roomClient)
					}
				}
			}
		}
	}
}

func HandleLeaveGroup(client *Client, roomIDStr string) {
	roomID := parseRoomID(roomIDStr)

	// Check if user is part of the room
	var roomUser models.RoomUser
	if err := database.DB.
		Where("room_id = ? AND user_id = ?", roomID, client.userID).
		First(&roomUser).Error; err != nil {
		log.Printf("User %d is not in room %d or already removed", client.userID, roomID)
		sendErrorToClient(client, "You are not a member of this group")
		return
	}

	// Remove user from database (room_users table)
	if err := database.DB.Delete(&roomUser).Error; err != nil {
		log.Printf("Error removing user %d from room %d: %v", client.userID, roomID, err)
		sendErrorToClient(client, "Failed to leave group")
		return
	}

	// Remove user from WebSocket room
	client.leaveRoom(roomID)
	log.Printf("User %d left group %d", client.userID, roomID)

	// Notify others in the room that the user has left
	leaveMsg := Message{
		Type: "user_left_group",
		Payload: map[string]interface{}{
			"user_id": client.userID,
			"room_id": roomID,
		},
	}

	msgBytes, err := json.Marshal(leaveMsg)
	if err != nil {
		log.Printf("Error marshaling leave group message: %v", err)
		return
	}

	client.hub.broadcastToRoom(roomID, msgBytes)

	// Optional: Send confirmation back to the user
	confirmation := Message{
		Type: "group_left",
		Payload: map[string]interface{}{
			"room_id": roomID,
			"message": "You have successfully left the group",
		},
	}

	if response, err := json.Marshal(confirmation); err == nil {
		client.send <- response
	}
}
