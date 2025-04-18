package controllers

import (
	"net/http"

	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/models"
	"github.com/gin-gonic/gin"
)

type SendInviteInput struct {
	RoomID   uint   `json:"room_id" binding:"required" example:"1"`
	Username string `json:"username" binding:"required" example:"johndoe"`
}

type RespondInviteInput struct {
	InviteID uint   `json:"invite_id" binding:"required" example:"1"`
	Action   string `json:"action" binding:"required,oneof=accept reject" example:"accept"`
}

// GetPendingInvites godoc
// @Summary Get pending invites for the authenticated user
// @Description Returns all pending invitations for the authenticated user
// @Tags invites
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "List of pending invites"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/invites/pending [get]
func GetPendingInvites(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var invites []models.InviteRequest
	if err := database.DB.Where("receiver_id = ? AND status = 'pending'", userID).
		Preload("Room").Preload("Sender").
		Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch invites"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"invites": invites})
}

// GetSentInvites godoc
// @Summary Get invites sent by the authenticated user
// @Description Returns all invitations sent by the authenticated user
// @Tags invites
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "List of sent invites"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/invites/sent [get]
func GetSentInvites(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var invites []models.InviteRequest
	if err := database.DB.Where("sender_id = ?", userID).
		Preload("Room").Preload("Receiver").
		Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch invites"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"invites": invites})
}

// SendInvite godoc
// @Summary Send an invitation to a user
// @Description Invites a user to join a chat room
// @Tags invites
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param invite body SendInviteInput true "Invite Creation"
// @Success 201 {object} map[string]interface{} "Invitation sent successfully"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/invites [post]
func SendInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input SendInviteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is in the room
	var roomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", input.RoomID, userID).
		First(&roomUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this room"})
		return
	}

	// Find the user to invite
	var receiver models.User
	if err := database.DB.Where("username = ?", input.Username).First(&receiver).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user is already in the room
	var existingRoomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", input.RoomID, receiver.ID).
		First(&existingRoomUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already a member of this room"})
		return
	}

	// Check if there's already a pending invite
	var existingInvite models.InviteRequest
	if err := database.DB.Where("room_id = ? AND receiver_id = ? AND status = 'pending'",
		input.RoomID, receiver.ID).First(&existingInvite).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "An invitation has already been sent to this user"})
		return
	}

	// Create invite
	invite := models.InviteRequest{
		RoomID:     input.RoomID,
		SenderID:   userID,
		ReceiverID: receiver.ID,
		Status:     "pending",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		return
	}

	// Load relationships for the response
	database.DB.Preload("Room").Preload("Receiver").First(&invite, invite.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Invitation sent successfully",
		"invite":  invite,
	})
}

// RespondToInvite godoc
// @Summary Respond to an invitation
// @Description Accept or reject an invitation to join a chat room
// @Tags invites
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param response body RespondInviteInput true "Invitation Response"
// @Success 200 {object} map[string]string "Response processed successfully"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 404 {object} map[string]string "Invitation not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/invites/respond [post]
func RespondToInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input RespondInviteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the invite
	var invite models.InviteRequest
	if err := database.DB.Where("id = ? AND receiver_id = ? AND status = 'pending'",
		input.InviteID, userID).First(&invite).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found or already processed"})
		return
	}

	if input.Action == "accept" {
		// Update invite status
		invite.Status = "accepted"
		if err := database.DB.Save(&invite).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invitation"})
			return
		}

		// Add user to room
		roomUser := models.RoomUser{
			RoomID: invite.RoomID,
			UserID: userID,
		}

		if err := database.DB.Create(&roomUser).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to room"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Invitation accepted successfully"})
	} else {
		// Update invite status
		invite.Status = "rejected"
		if err := database.DB.Save(&invite).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invitation"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Invitation rejected successfully"})
	}
}
