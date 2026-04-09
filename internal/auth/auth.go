package auth

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	svcerrors "project-serverless/internal/errors"
)

type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(password string, passwordHash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

func GenerateToken(userID int, email string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", svcerrors.Internal("JWT_SECRET is required", nil)
	}

	now := time.Now().UTC()
	ttl := time.Duration(getJWTTTLMinutes()) * time.Minute
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   strconv.Itoa(userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString string) (*Claims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, svcerrors.Internal("JWT_SECRET is required", nil)
	}

	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, svcerrors.Unauthorized("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, svcerrors.Unauthorized("invalid token")
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, svcerrors.Unauthorized("invalid token")
	}
	return claims, nil
}

func ExtractBearerToken(header string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", svcerrors.Unauthorized("invalid authorization header")
	}
	return strings.TrimSpace(parts[1]), nil
}

func AuthorizeHeader(header string) (*Claims, error) {
	token, err := ExtractBearerToken(header)
	if err != nil {
		return nil, err
	}
	return ValidateToken(token)
}

func getJWTTTLMinutes() int {
	raw := os.Getenv("JWT_TTL_MINUTES")
	if raw == "" {
		return 60
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 60
	}
	return v
}
