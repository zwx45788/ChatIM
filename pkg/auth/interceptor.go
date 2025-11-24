package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type UserId string

var user_id UserId = "user_id"

// UnaryAuthInterceptor 是一个一元拦截器，用于验证 JWT Token
func UnaryAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 1. 从 metadata 中获取 authorization token
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Missing metadata")
	}
	authHeaders := md["authorization"]
	if len(authHeaders) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "Missing authorization token")
	}

	// 2. 解析和验证 Token
	tokenString := authHeaders[0]
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	claims, err := ParseToken(tokenString)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid token: %v", err)
	}

	// 3. 将解析出的 user_id 放回 context，方便后续的 handler 使用
	// 这样 handler 就可以不用再解析了，直接从 context 取
	newCtx := context.WithValue(ctx, user_id, claims.UserID)

	// 4. 继续执行后续的 handler，并传入带有 user_id 的新 context
	return handler(newCtx, req)
}
