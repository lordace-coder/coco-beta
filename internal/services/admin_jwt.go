package services

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func adminSecret() []byte {
	s := os.Getenv("SECRET")
	if s == "" {
		s = "change-me-in-production"
	}
	return []byte("admin:" + s)
}

// CreateAdminToken creates a signed JWT for a dashboard admin user.
func CreateAdminToken(adminID, email string) (string, error) {
	claims := jwt.MapClaims{
		"adminId": adminID,
		"email":   email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(adminSecret())
}

// DecodeAdminToken validates and decodes an admin JWT.
func DecodeAdminToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return adminSecret(), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return claims, nil
}
