package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// GenerateToken creates a new JWT token for a user
func GenerateToken(userID uint) (string, error) {
	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key" // Default secret (not recommended for production)
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // Token expires in 7 days
	})

	// Sign and get the complete encoded token as a string
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
