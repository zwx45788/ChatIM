package middleware

import (
	"net/http"
	"strings"

	"ChatIM/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware JWT 认证中间件
type contextKey string

const UserIDKey contextKey = "userID"

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 从 Header 中获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort() // 阻止后续处理
			return
		}

		// 2. 检查 Token 格式 ("Bearer <token>")
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		// 3. 解析 Token
		tokenString := parts[1]
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 4. 将 Token 中的信息（如 userID）存入 gin.Context，以便后续的 handler 使用
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
