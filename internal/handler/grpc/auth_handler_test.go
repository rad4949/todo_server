package grpchandler

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	authv1 "todo_server/internal/gen/auth/v1"
	"todo_server/internal/model"
	"todo_server/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAuthUserService struct {
	authenticateFunc func(username, password string) (*model.User, error)
}

func (f *fakeAuthUserService) Authenticate(username, password string) (*model.User, error) {
	if f.authenticateFunc != nil {
		return f.authenticateFunc(username, password)
	}

	return nil, errors.New("invalid credentials")
}

type fakeAuthJWTService struct {
	generateAccessTokenFunc  func(userID string, username string) (string, error)
	generateRefreshTokenFunc func(userID string, username string) (string, error)
	validateRefreshTokenFunc func(refreshToken string) (*service.Claims, error)
}

func (f *fakeAuthJWTService) GenerateAccessToken(userID string, username string) (string, error) {
	if f.generateAccessTokenFunc != nil {
		return f.generateAccessTokenFunc(userID, username)
	}

	return "access-token", nil
}

func (f *fakeAuthJWTService) GenerateRefreshToken(userID string, username string) (string, error) {
	if f.generateRefreshTokenFunc != nil {
		return f.generateRefreshTokenFunc(userID, username)
	}

	return "refresh-token", nil
}

func (f *fakeAuthJWTService) ValidateRefreshToken(refreshToken string) (*service.Claims, error) {
	if f.validateRefreshTokenFunc != nil {
		return f.validateRefreshTokenFunc(refreshToken)
	}

	return &service.Claims{
		UserID:   "user-1",
		Username: "grpc_user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, nil
}

type fakeAuthBlocklist struct {
	blockFunc     func(ctx context.Context, token string, ttl time.Duration) error
	isBlockedFunc func(ctx context.Context, token string) bool
}

func (f *fakeAuthBlocklist) Block(ctx context.Context, token string, ttl time.Duration) error {
	if f.blockFunc != nil {
		return f.blockFunc(ctx, token, ttl)
	}

	return nil
}

func (f *fakeAuthBlocklist) IsBlocked(ctx context.Context, token string) bool {
	if f.isBlockedFunc != nil {
		return f.isBlockedFunc(ctx, token)
	}

	return false
}

func newAuthTestClient(
	t *testing.T,
	jwtService AuthJWTService,
	userService AuthUserService,
	blocklist AuthBlocklist,
) authv1.AuthServiceClient {
	t.Helper()

	listener := bufconn.Listen(bufSize)

	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, NewAuthHandler(jwtService, userService, blocklist))

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("failed to serve grpc test server: %v", err)
		}
	}()

	t.Cleanup(func() {
		server.Stop()
		listener.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return authv1.NewAuthServiceClient(conn)
}

func TestAuthHandler_Login_Success(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{
			generateAccessTokenFunc: func(userID string, username string) (string, error) {
				if userID != "user-1" {
					t.Fatalf("expected userID user-1, got %s", userID)
				}

				if username != "grpc_user" {
					t.Fatalf("expected username grpc_user, got %s", username)
				}

				return "access-token", nil
			},
			generateRefreshTokenFunc: func(userID string, username string) (string, error) {
				return "refresh-token", nil
			},
		},
		&fakeAuthUserService{
			authenticateFunc: func(username, password string) (*model.User, error) {
				if username != "grpc_user" {
					t.Fatalf("expected username grpc_user, got %s", username)
				}

				if password != "123456" {
					t.Fatalf("expected password 123456, got %s", password)
				}

				return &model.User{
					ID:       "user-1",
					Username: "grpc_user",
					Email:    "grpc_user@example.com",
				}, nil
			},
		},
		&fakeAuthBlocklist{},
	)

	resp, err := client.Login(context.Background(), &authv1.LoginRequest{
		Username: "grpc_user",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetAccessToken() != "access-token" {
		t.Fatalf("expected access-token, got %s", resp.GetAccessToken())
	}

	if resp.GetRefreshToken() != "refresh-token" {
		t.Fatalf("expected refresh-token, got %s", resp.GetRefreshToken())
	}
}

func TestAuthHandler_Login_EmptyUsername(t *testing.T) {
	client := newAuthTestClient(t, &fakeAuthJWTService{}, &fakeAuthUserService{}, &fakeAuthBlocklist{})

	_, err := client.Login(context.Background(), &authv1.LoginRequest{
		Username: "",
		Password: "123456",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestAuthHandler_Login_EmptyPassword(t *testing.T) {
	client := newAuthTestClient(t, &fakeAuthJWTService{}, &fakeAuthUserService{}, &fakeAuthBlocklist{})

	_, err := client.Login(context.Background(), &authv1.LoginRequest{
		Username: "grpc_user",
		Password: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{},
		&fakeAuthUserService{
			authenticateFunc: func(username, password string) (*model.User, error) {
				return nil, errors.New("invalid credentials")
			},
		},
		&fakeAuthBlocklist{},
	)

	_, err := client.Login(context.Background(), &authv1.LoginRequest{
		Username: "grpc_user",
		Password: "wrong",
	})

	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{
			validateRefreshTokenFunc: func(refreshToken string) (*service.Claims, error) {
				if refreshToken != "refresh-token" {
					t.Fatalf("expected refresh-token, got %s", refreshToken)
				}

				return &service.Claims{
					UserID:   "user-1",
					Username: "grpc_user",
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					},
				}, nil
			},
			generateAccessTokenFunc: func(userID string, username string) (string, error) {
				return "new-access-token", nil
			},
		},
		&fakeAuthUserService{},
		&fakeAuthBlocklist{},
	)

	resp, err := client.Refresh(context.Background(), &authv1.RefreshRequest{
		RefreshToken: "refresh-token",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetAccessToken() != "new-access-token" {
		t.Fatalf("expected new-access-token, got %s", resp.GetAccessToken())
	}
}

func TestAuthHandler_Refresh_EmptyToken(t *testing.T) {
	client := newAuthTestClient(t, &fakeAuthJWTService{}, &fakeAuthUserService{}, &fakeAuthBlocklist{})

	_, err := client.Refresh(context.Background(), &authv1.RefreshRequest{
		RefreshToken: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestAuthHandler_Refresh_BlockedToken(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{},
		&fakeAuthUserService{},
		&fakeAuthBlocklist{
			isBlockedFunc: func(ctx context.Context, token string) bool {
				return true
			},
		},
	)

	_, err := client.Refresh(context.Background(), &authv1.RefreshRequest{
		RefreshToken: "refresh-token",
	})

	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{
			validateRefreshTokenFunc: func(refreshToken string) (*service.Claims, error) {
				return nil, errors.New("invalid token")
			},
		},
		&fakeAuthUserService{},
		&fakeAuthBlocklist{},
	)

	_, err := client.Refresh(context.Background(), &authv1.RefreshRequest{
		RefreshToken: "invalid-token",
	})

	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{
			validateRefreshTokenFunc: func(refreshToken string) (*service.Claims, error) {
				return &service.Claims{
					UserID:   "user-1",
					Username: "grpc_user",
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					},
				}, nil
			},
		},
		&fakeAuthUserService{},
		&fakeAuthBlocklist{
			blockFunc: func(ctx context.Context, token string, ttl time.Duration) error {
				if token != "refresh-token" {
					t.Fatalf("expected refresh-token, got %s", token)
				}

				if ttl <= 0 {
					t.Fatalf("expected positive ttl, got %v", ttl)
				}

				return nil
			},
		},
	)

	resp, err := client.Logout(context.Background(), &authv1.LogoutRequest{
		RefreshToken: "refresh-token",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !resp.GetSuccess() {
		t.Fatalf("expected success true")
	}
}

func TestAuthHandler_Logout_EmptyToken(t *testing.T) {
	client := newAuthTestClient(t, &fakeAuthJWTService{}, &fakeAuthUserService{}, &fakeAuthBlocklist{})

	_, err := client.Logout(context.Background(), &authv1.LogoutRequest{
		RefreshToken: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestAuthHandler_Logout_InvalidToken(t *testing.T) {
	client := newAuthTestClient(
		t,
		&fakeAuthJWTService{
			validateRefreshTokenFunc: func(refreshToken string) (*service.Claims, error) {
				return nil, errors.New("invalid token")
			},
		},
		&fakeAuthUserService{},
		&fakeAuthBlocklist{},
	)

	_, err := client.Logout(context.Background(), &authv1.LogoutRequest{
		RefreshToken: "invalid-token",
	})

	assertGRPCCode(t, err, codes.Unauthenticated)
}
