package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	usedMFAChallenges sync.Map
	ephemeralJWTOnce  sync.Once
	ephemeralJWTKey   []byte
)

func isWeakJWTSecret(sec string) bool {
	s := strings.TrimSpace(sec)
	return s == "" || s == "joss_default_secret_change_in_production" || s == "joss_default_secret"
}

func (r *Runtime) resolveJWTSecret() []byte {
	if r != nil && r.Env != nil {
		if sec := r.Env["JWT_SECRET"]; !isWeakJWTSecret(sec) {
			return []byte(sec)
		}
	}
	if sec := os.Getenv("JWT_SECRET"); !isWeakJWTSecret(sec) {
		return []byte(sec)
	}
	if r != nil && r.Env != nil {
		if sec := r.Env["APP_KEY"]; !isWeakJWTSecret(sec) {
			return []byte(sec)
		}
	}
	if sec := os.Getenv("APP_KEY"); !isWeakJWTSecret(sec) {
		return []byte(sec)
	}
	ephemeralJWTOnce.Do(func() {
		raw := make([]byte, 32)
		_, _ = rand.Read(raw)
		ephemeralJWTKey = []byte(hex.EncodeToString(raw))
	})
	return ephemeralJWTKey
}

func claimUserID(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		var userId int
		fmt.Sscanf(fmt.Sprintf("%v", typed), "%d", &userId)
		return userId
	}
}

func (r *Runtime) generateJWT(userId int, email string, userName string, roleName string, isRefresh bool) interface{} {
	expirationTime := time.Now().Add(24 * 30 * time.Hour)
	tokenType := "access"
	if isRefresh {
		expirationTime = time.Now().Add(24 * 180 * time.Hour)
		tokenType = "refresh"
	}

	jwtSecret := r.resolveJWTSecret()

	claims := jwt.MapClaims{
		"user_id":    userId,
		"email":      email,
		"name":       userName,
		"role":       roleName,
		"token_type": tokenType,
		"exp":        expirationTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		fmt.Printf("[Security] Error generando JWT: %v\n", err)
		return false
	}

	return tokenString
}

func (r *Runtime) generateMFAChallengeJWT(userId int, email string) interface{} {
	usedMFAChallenges.Range(func(key, value interface{}) bool {
		if usedAt, ok := value.(time.Time); ok && time.Since(usedAt) > 10*time.Minute {
			usedMFAChallenges.Delete(key)
		}
		return true
	})

	jwtSecret := r.resolveJWTSecret()

	claims := jwt.MapClaims{
		"user_id":    userId,
		"email":      email,
		"name":       "MFA_Pending",
		"role":       "guest",
		"token_type": "mfa_challenge",
		"jti":        uuid.New().String(),
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		LogError("[Security] Error generating MFA challenge: %v", err)
		return false
	}
	return tokenString
}

func (r *Runtime) parseJWT(tokenString string) (map[string]interface{}, bool) {
	jwtSecret := r.resolveJWTSecret()

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		fmt.Printf("[ValidateJWT Error] Token Length: %d | Error: %v\n", len(tokenString), err)
		return nil, false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, true
	}

	return nil, false
}

func (r *Runtime) ValidateJWT(tokenString string) (map[string]interface{}, bool) {
	claims, valid := r.parseJWT(tokenString)
	if !valid {
		return nil, false
	}
	if fmt.Sprintf("%v", claims["token_type"]) == "mfa_challenge" {
		return nil, false
	}
	return claims, true
}
