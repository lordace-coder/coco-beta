package services

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/patrick/cocobase/internal/models"
)

var (
	// JWT secret key - should be loaded from environment in production
	jwtSecretKey = []byte("your-secret-key-change-this-in-production")

	// Token expiration time
	tokenExpiration = 24 * time.Hour * 30 // 30 days
)

// AppUserClaims represents JWT claims for app users
type AppUserClaims struct {
	UserID   string `json:"user_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// CreateAppUserToken signs with the user's client_id as the secret (matches Python)
func CreateAppUserToken(user *models.AppUser) (string, error) {
	claims := jwt.MapClaims{
		"userId": user.ID,
		"email":  user.Email,
		"exp":    time.Now().UTC().Add(48 * time.Hour).Unix(), // 2 days like Python
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(user.ClientID)) // client_id is the secret
}

// DecodeAppUserToken validates using the projectId as the secret (matches Python)
func DecodeAppUserToken(tokenString string, projectID string) (jwt.MapClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(projectID), nil // projectId is the secret
	})
	if err != nil {
		// Differentiate expired vs invalid
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token has expired")
		}
		return nil, errors.New("invalid token")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// SetJWTSecret sets the JWT secret key (should be called at app startup)
func SetJWTSecret(secret string) {
	jwtSecretKey = []byte(secret)
}
