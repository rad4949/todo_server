package grpchandler

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	userv1 "todo_server/internal/gen/user/v1"
	"todo_server/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type fakeUserService struct {
	registerFunc func(username, email, password string) (model.User, error)
	getByIDFunc  func(id string) (*model.User, error)
	getAllFunc   func() ([]model.User, error)
}

func (f *fakeUserService) Register(username, email, password string) (model.User, error) {
	if f.registerFunc != nil {
		return f.registerFunc(username, email, password)
	}

	return model.User{}, nil
}

func (f *fakeUserService) GetByID(id string) (*model.User, error) {
	if f.getByIDFunc != nil {
		return f.getByIDFunc(id)
	}

	return nil, errors.New("not found")
}

func (f *fakeUserService) GetAll() ([]model.User, error) {
	if f.getAllFunc != nil {
		return f.getAllFunc()
	}

	return []model.User{}, nil
}

func newUserTestClient(t *testing.T, userService UserService) userv1.UserServiceClient {
	t.Helper()

	listener := bufconn.Listen(bufSize)

	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, NewUserHandler(userService))

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

	return userv1.NewUserServiceClient(conn)
}

func TestUserHandler_Register_Success(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{
		registerFunc: func(username, email, password string) (model.User, error) {
			if username != "grpc_user" {
				t.Fatalf("expected username grpc_user, got %s", username)
			}

			if email != "grpc_user@example.com" {
				t.Fatalf("expected email grpc_user@example.com, got %s", email)
			}

			if password != "123456" {
				t.Fatalf("expected password 123456, got %s", password)
			}

			return model.User{
				ID:        "user-1",
				Username:  username,
				Email:     email,
				CreatedAt: time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
			}, nil
		},
	})

	resp, err := client.Register(context.Background(), &userv1.RegisterRequest{
		Username: "grpc_user",
		Email:    "grpc_user@example.com",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetUser().GetId() != "user-1" {
		t.Fatalf("expected user id user-1, got %s", resp.GetUser().GetId())
	}

	if resp.GetUser().GetUsername() != "grpc_user" {
		t.Fatalf("expected username grpc_user, got %s", resp.GetUser().GetUsername())
	}

	if resp.GetUser().GetEmail() != "grpc_user@example.com" {
		t.Fatalf("expected email grpc_user@example.com, got %s", resp.GetUser().GetEmail())
	}
}

func TestUserHandler_Register_EmptyUsername(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{})

	_, err := client.Register(context.Background(), &userv1.RegisterRequest{
		Username: "",
		Email:    "grpc_user@example.com",
		Password: "123456",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestUserHandler_Register_EmptyEmail(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{})

	_, err := client.Register(context.Background(), &userv1.RegisterRequest{
		Username: "grpc_user",
		Email:    "",
		Password: "123456",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestUserHandler_Register_EmptyPassword(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{})

	_, err := client.Register(context.Background(), &userv1.RegisterRequest{
		Username: "grpc_user",
		Email:    "grpc_user@example.com",
		Password: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestUserHandler_GetUser_Success(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{
		getByIDFunc: func(id string) (*model.User, error) {
			if id != "user-1" {
				t.Fatalf("expected id user-1, got %s", id)
			}

			return &model.User{
				ID:        "user-1",
				Username:  "grpc_user",
				Email:     "grpc_user@example.com",
				CreatedAt: time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
			}, nil
		},
	})

	resp, err := client.GetUser(context.Background(), &userv1.GetUserRequest{
		Id: "user-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetUser().GetId() != "user-1" {
		t.Fatalf("expected user id user-1, got %s", resp.GetUser().GetId())
	}
}

func TestUserHandler_GetUser_EmptyID(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{})

	_, err := client.GetUser(context.Background(), &userv1.GetUserRequest{
		Id: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestUserHandler_ListUsers_Success(t *testing.T) {
	client := newUserTestClient(t, &fakeUserService{
		getAllFunc: func() ([]model.User, error) {
			return []model.User{
				{
					ID:        "user-1",
					Username:  "grpc_user_1",
					Email:     "grpc_user_1@example.com",
					CreatedAt: time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
				},
				{
					ID:        "user-2",
					Username:  "grpc_user_2",
					Email:     "grpc_user_2@example.com",
					CreatedAt: time.Date(2026, 6, 25, 10, 5, 0, 0, time.UTC),
				},
			}, nil
		},
	})

	resp, err := client.ListUsers(context.Background(), &userv1.ListUsersRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.GetUsers()) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.GetUsers()))
	}
}

func assertGRPCCode(t *testing.T, err error, expected codes.Code) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error with code %s, got nil", expected)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got %v", err)
	}

	if st.Code() != expected {
		t.Fatalf("expected grpc code %s, got %s", expected, st.Code())
	}
}
