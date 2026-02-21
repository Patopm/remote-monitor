package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Patopm/remote-monitor/internal/protocol"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecretKey = []byte("super-secret-key-change-me")

// GenerateJWT creates a new token for a valid user
func GenerateJWT(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 hours
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecretKey)
}

// AuthMiddleware intercepts requests and validates the JWT token
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, protocol.APIResponse{
				Success: false,
				Message: "Missing Authorization header",
			})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeJSON(w, http.StatusUnauthorized, protocol.APIResponse{
				Success: false,
				Message: "Invalid Authorization format",
			})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecretKey, nil
		})

		if err != nil || !token.Valid {
			writeJSON(w, http.StatusUnauthorized, protocol.APIResponse{
				Success: false,
				Message: "Invalid or expired token",
			})
			return
		}

		next.ServeHTTP(w, r)
	}
}
