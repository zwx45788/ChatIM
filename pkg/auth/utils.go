package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var jwtSecretKey = []byte("your-super-secret-key-that-is-long-and-random")

type JWTClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID string) (string, error) {
	// ... (保持不变) ...
	claims := &JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecretKey)
}

func ExtractToken(ctx context.Context) (string, error) {
	// ... (保持不变) ...
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "Missing metadata")
	}
	authHeaders := md["authorization"]
	if len(authHeaders) == 0 {
		return "", status.Errorf(codes.Unauthenticated, "Missing authorization token")
	}
	tokenString := authHeaders[0]
	return strings.TrimPrefix(tokenString, "Bearer "), nil
}

// ParseToken 解析并验证 Token，如果成功，返回 Claims
func ParseToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetUserID 从 context 中提取、验证 Token 并返回 UserID
func GetUserID(ctx context.Context) (string, error) {
	tokenString, err := ExtractToken(ctx)
	if err != nil {
		return "", err
	}

	claims, err := ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	return claims.UserID, nil
}
