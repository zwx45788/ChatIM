package middleware

import (
	"log"
	"net/http"
	"strings"

	"ChatIM/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// AuthMiddleware JWT 认证中间件
type contextKey string

const UserIDKey contextKey = "userID"

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 1. 优先从 Header 中获取 Token (用于普通API请求)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// 检查 Token 格式 ("Bearer <token>")
			parts := strings.SplitN(authHeader, " ", 2)
			if !(len(parts) == 2 && parts[0] == "Bearer") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
				c.Abort()
				return
			}
			tokenString = parts[1]
		} else {
			// 2. 如果 Header 中没有，则从 URL 查询参数中获取 (用于WebSocket连接)
			tokenString = c.Query("token")
		}

		// 3. 检查是否最终获取到了Token
		if tokenString == "" {
			// 根据请求类型返回不同格式的错误
			// 对于WebSocket升级请求，返回JSON可能不合适，直接返回状态码更标准
			if websocket.IsWebSocketUpgrade(c.Request) {
				c.AbortWithStatus(http.StatusUnauthorized)
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token is required"})
			}
			return
		}

		// 4. 解析 Token
		claims, err := auth.ParseToken(tokenString)
		log.Printf("Attempting to parse token: %s", tokenString)
		if err != nil {
			// 同样，根据请求类型返回错误
			log.Printf("Token parsing failed: %v", err)
			if websocket.IsWebSocketUpgrade(c.Request) {
				c.AbortWithStatus(http.StatusUnauthorized)
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			}
			return
		}

		// 5. 将 Token 中的信息（如 userID）存入 gin.Context
		c.Set("userID", claims.UserID)
		c.Next() // 继续执行后续的中间件或 handler
	}
}

func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	id, ok := userID.(string)
	return id, ok
}
