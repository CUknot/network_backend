package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/models"
)

// InvitePayload represents the structure of an invite payload
type InvitePayload struct {
	RoomID   uint   `json:"room_id"`
	Username string `json:"username"`
	Tag      string `json:"tag"`
}

// HandleInviteUsers processes an invitation request
func HandleInviteUsers(client *Client, payload InvitePayload) {
	// Check if user is in the room
	if !client.inRoom(payload.RoomID) {
		log.Printf("User %d attempted to invite to room %d without being a member",
			client.userID, payload.RoomID)
		sendErrorToClient(client, "You must be a member of the room to invite others")
		return
	}

	// Find the user to invite by username and tag
	var receiver models.User
	if err := database.DB.Where("username = ?", payload.Username).First(&receiver).Error; err != nil {
		log.Printf("User not found: %s", payload.Username)
		sendErrorToClient(client, "User not found")
		return
	}

	// Check if user is already in the room
	var existingRoomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", payload.RoomID, receiver.ID).
		First(&existingRoomUser).Error; err == nil {
		log.Printf("User %d is already in room %d", receiver.ID, payload.RoomID)
		sendErrorToClient(client, "User is already a member of this room")
		return
	}

	// Check if there's already a pending invite
	var existingInvite models.InviteRequest
	if err := database.DB.Where("room_id = ? AND receiver_id = ? AND status = 'pending'",
		payload.RoomID, receiver.ID).First(&existingInvite).Error; err == nil {
		log.Printf("Invite already exists for user %d to room %d", receiver.ID, payload.RoomID)
		sendErrorToClient(client, "An invitation has already been sent to this user")
		return
	}

	// Get room details
	var room models.Room
	if err := database.DB.First(&room, payload.RoomID).Error; err != nil {
		log.Printf("Room not found: %d", payload.RoomID)
		sendErrorToClient(client, "Room not found")
		return
	}

	// Create invite request
	invite := models.InviteRequest{
		RoomID:     payload.RoomID,
		SenderID:   client.userID,
		ReceiverID: receiver.ID,
		Status:     "pending",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		log.Printf("Error creating invite: %v", err)
		sendErrorToClient(client, "Failed to create invitation")
		return
	}

	// Load relationships for the response
	database.DB.Preload("Room").Preload("Sender").First(&invite, invite.ID)

	// Notify the invited user if they're online
	notifyUserOfInvite(receiver.ID, invite)

	// Send confirmation to the inviter
	sendInviteConfirmation(client, invite)
}

// HandleAcceptInvite processes an invitation acceptance
func HandleAcceptInvite(client *Client, roomIDStr string) {
	roomID := parseRoomID(roomIDStr)

	// Find the invite
	var invite models.InviteRequest
	if err := database.DB.Where("room_id = ? AND receiver_id = ? AND status = 'pending'",
		roomID, client.userID).First(&invite).Error; err != nil {
		log.Printf("Invite not found for user %d to room %d", client.userID, roomID)
		sendErrorToClient(client, "Invitation not found or already processed")
		return
	}

	// Update invite status
	invite.Status = "accepted"
	if err := database.DB.Save(&invite).Error; err != nil {
		log.Printf("Error updating invite: %v", err)
		sendErrorToClient(client, "Failed to accept invitation")
		return
	}

	// Add user to room
	roomUser := models.RoomUser{
		RoomID:     roomID,
		UserID:     client.userID,
		LastReadAt: time.Now(),
	}

	if err := database.DB.Create(&roomUser).Error; err != nil {
		log.Printf("Error adding user to room: %v", err)
		sendErrorToClient(client, "Failed to join room")
		return
	}

	// Join the room in the WebSocket connection
	client.joinRoom(roomID)

	// Get room details
	var room models.Room
	database.DB.Preload("Users").First(&room, roomID)

	// Notify the room of the new member
	notifyRoomOfNewMember(roomID, client.userID)

	// Send confirmation to the user
	sendRoomJoinConfirmation(client, room)
}

// HandleRejectInvite processes an invitation rejection
func HandleRejectInvite(client *Client, roomIDStr string) {
	roomID := parseRoomID(roomIDStr)

	// Find the invite
	var invite models.InviteRequest
	if err := database.DB.Where("room_id = ? AND receiver_id = ? AND status = 'pending'",
		roomID, client.userID).First(&invite).Error; err != nil {
		log.Printf("Invite not found for user %d to room %d", client.userID, roomID)
		sendErrorToClient(client, "Invitation not found or already processed")
		return
	}

	// Update invite status
	invite.Status = "rejected"
	if err := database.DB.Save(&invite).Error; err != nil {
		log.Printf("Error updating invite: %v", err)
		sendErrorToClient(client, "Failed to reject invitation")
		return
	}

	// Notify the inviter if they're online
	notifyInviterOfRejection(invite)

	// Send confirmation to the user
	sendRejectConfirmation(client, invite)
}

// Helper functions

func notifyUserOfInvite(userID uint, invite models.InviteRequest) {
	// Find the client for this user
	for client := range hub.clients {
		if client.userID == userID {
			// Create notification message
			notification := Message{
				Type: "invite_received",
				Payload: map[string]interface{}{
					"invite_id": invite.ID,
					"room_id":   invite.RoomID,
					"room_name": invite.Room.Name,
					"sender":    invite.Sender.Username,
				},
			}

			// Send to client
			notificationBytes, _ := json.Marshal(notification)
			client.send <- notificationBytes
			break
		}
	}
}

func notifyRoomOfNewMember(roomID uint, userID uint) {
	// Get user details
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		log.Printf("Error fetching user %d: %v", userID, err)
		return
	}

	// Create notification message
	notification := Message{
		Type: "user_joined",
		Payload: map[string]interface{}{
			"room_id":  roomID,
			"user_id":  userID,
			"username": user.Username,
		},
	}

	// Broadcast to room
	notificationBytes, _ := json.Marshal(notification)
	hub.broadcastToRoom(roomID, notificationBytes)
}

func notifyInviterOfRejection(invite models.InviteRequest) {
	// Find the client for the inviter
	for client := range hub.clients {
		if client.userID == invite.SenderID {
			// Create notification message
			notification := Message{
				Type: "invite_rejected",
				Payload: map[string]interface{}{
					"invite_id": invite.ID,
					"room_id":   invite.RoomID,
					"user_id":   invite.ReceiverID,
				},
			}

			// Send to client
			notificationBytes, _ := json.Marshal(notification)
			client.send <- notificationBytes
			break
		}
	}
}

func sendErrorToClient(client *Client, errorMessage string) {
	errorMsg := Message{
		Type: "error",
		Payload: map[string]string{
			"message": errorMessage,
		},
	}

	errorBytes, _ := json.Marshal(errorMsg)
	client.send <- errorBytes
}

func sendInviteConfirmation(client *Client, invite models.InviteRequest) {
	confirmation := Message{
		Type: "invite_sent",
		Payload: map[string]interface{}{
			"invite_id":   invite.ID,
			"room_id":     invite.RoomID,
			"room_name":   invite.Room.Name,
			"receiver_id": invite.ReceiverID,
		},
	}

	confirmationBytes, _ := json.Marshal(confirmation)
	client.send <- confirmationBytes
}

func sendRoomJoinConfirmation(client *Client, room models.Room) {
	confirmation := Message{
		Type:    "room_joined",
		Payload: room,
	}

	confirmationBytes, _ := json.Marshal(confirmation)
	client.send <- confirmationBytes
}

func sendRejectConfirmation(client *Client, invite models.InviteRequest) {
	confirmation := Message{
		Type: "invite_rejected_confirmation",
		Payload: map[string]interface{}{
			"invite_id": invite.ID,
			"room_id":   invite.RoomID,
		},
	}

	confirmationBytes, _ := json.Marshal(confirmation)
	client.send <- confirmationBytes
}
