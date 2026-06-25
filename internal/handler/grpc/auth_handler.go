package grpchandler

import (
	"context"
	"strings"
	"time"

	authv1 "todo_server/internal/gen/auth/v1"
	"todo_server/internal/model"
	"todo_server/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthUserService interface {
	Authenticate(username, password string) (*model.User, error)
}

type AuthJWTService interface {
	GenerateAccessToken(userID string, username string) (string, error)
	GenerateRefreshToken(userID string, username string) (string, error)
	ValidateRefreshToken(refreshToken string) (*service.Claims, error)
}

type AuthBlocklist interface {
	Block(ctx context.Context, token string, ttl time.Duration) error
	IsBlocked(ctx context.Context, token string) bool
}

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	jwtService  AuthJWTService
	userService AuthUserService
	blocklist   AuthBlocklist
}

func NewAuthHandler(
	jwtService AuthJWTService,
	userService AuthUserService,
	blocklist AuthBlocklist,
) *AuthHandler {
	return &AuthHandler{
		jwtService:  jwtService,
		userService: userService,
		blocklist:   blocklist,
	}
}

func (h *AuthHandler) Login(
	ctx context.Context,
	req *authv1.LoginRequest,
) (*authv1.LoginResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	password := strings.TrimSpace(req.GetPassword())

	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	if password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	user, err := h.userService.Authenticate(username, password)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	accessToken, err := h.jwtService.GenerateAccessToken(user.ID, user.Username)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, user.Username)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	return &authv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (h *AuthHandler) Refresh(
	ctx context.Context,
	req *authv1.RefreshRequest,
) (*authv1.RefreshResponse, error) {
	refreshToken := strings.TrimSpace(req.GetRefreshToken())
	if refreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	if h.blocklist.IsBlocked(ctx, refreshToken) {
		return nil, status.Error(codes.Unauthenticated, "refresh token is revoked")
	}

	claims, err := h.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	accessToken, err := h.jwtService.GenerateAccessToken(claims.UserID, claims.Username)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	return &authv1.RefreshResponse{
		AccessToken: accessToken,
	}, nil
}

func (h *AuthHandler) Logout(
	ctx context.Context,
	req *authv1.LogoutRequest,
) (*authv1.LogoutResponse, error) {
	refreshToken := strings.TrimSpace(req.GetRefreshToken())
	if refreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	claims, err := h.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil, status.Error(codes.Unauthenticated, "refresh token is expired")
	}

	if err := h.blocklist.Block(ctx, refreshToken, ttl); err != nil {
		return nil, status.Error(codes.Internal, "failed to logout")
	}

	return &authv1.LogoutResponse{
		Success: true,
	}, nil
}
