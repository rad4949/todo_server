package interceptors

import (
	"context"
	"strings"

	"todo_server/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	UserIDKey   contextKey = "userID"
	UsernameKey contextKey = "username"
)

type JWTService interface {
	ValidateAccessToken(token string) (*service.Claims, error)
}

func AuthUnaryInterceptor(jwtService JWTService) grpc.UnaryServerInterceptor {
	publicMethods := map[string]bool{
		"/auth.v1.AuthService/Login":    true,
		"/auth.v1.AuthService/Refresh":  true,
		"/auth.v1.AuthService/Logout":   true,
		"/user.v1.UserService/Register": true,
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
		}

		authHeader := values[0]
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "empty token")
		}

		claims, err := jwtService.ValidateAccessToken(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}

		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UsernameKey, claims.Username)

		return handler(ctx, req)
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(UsernameKey).(string)
	return username, ok
}
