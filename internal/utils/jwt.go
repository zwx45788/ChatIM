package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 定义一个密钥，在生产环境中应该从环境变量或配置文件中读取，绝对不能硬编码！
var jwtSecretKey = []byte("your-super-secret-key-that-is-long-and-random") // 👈 记得改成一个更复杂的密钥

// JWT Claims 自定义的载荷
type JWTClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT
func GenerateToken(userID string) (string, error) {
	// 创建 Claims
	claims := &JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			// 过期时间：24小时后
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			// 签发时间
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	// 使用 HS256 算法创建 Token 对象
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 使用密钥签名并获取完整的 JWT 字符串
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ParseToken 解析和验证 JWT
func ParseToken(tokenString string) (*JWTClaims, error) {
	// 解析 Token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法是否正确
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	// 验证 Token 是否有效，并提取 Claims
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
