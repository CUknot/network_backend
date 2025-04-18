package main

import (
	"log"
	"os"

	"github.com/CUknot/network_backend/controllers"
	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/middleware"
	"github.com/CUknot/network_backend/websocket"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize database
	database.Connect()
	database.Migrate()

	// Set up router
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Authentication routes
	auth := router.Group("/api")
	{
		auth.POST("/register", controllers.Register)
		auth.POST("/login", controllers.Login)
	}

	// Protected routes
	api := router.Group("/api")
	api.Use(middleware.JWTAuth())
	{
		// Room routes
		api.GET("/rooms", controllers.GetRooms)
		api.POST("/rooms", controllers.CreateRoom)
		api.GET("/rooms/:id", controllers.GetRoom)
		api.PUT("/rooms/:id", controllers.UpdateRoom)
		api.DELETE("/rooms/:id", controllers.DeleteRoom)
		api.GET("/rooms/:id/unread", controllers.GetUnreadCount)

		// Message routes
		api.GET("/messages", controllers.GetMessages)
		api.POST("/messages", controllers.CreateMessage)
	}

	// WebSocket route
	router.GET("/ws", websocket.HandleConnection)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
