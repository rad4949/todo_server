package grpchandler

import (
	"context"
	"strings"
	"time"

	userv1 "todo_server/internal/gen/user/v1"
	"todo_server/internal/model"
	"todo_server/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

func (h *UserHandler) Register(
	ctx context.Context,
	req *userv1.RegisterRequest,
) (*userv1.UserResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	email := strings.TrimSpace(req.GetEmail())
	password := strings.TrimSpace(req.GetPassword())

	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	if email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	if password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	user, err := h.service.Register(username, email, password)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &userv1.UserResponse{
		User: mapUserToProto(user),
	}, nil
}

func (h *UserHandler) GetUser(
	ctx context.Context,
	req *userv1.GetUserRequest,
) (*userv1.UserResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	user, err := h.service.GetByID(id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &userv1.UserResponse{
		User: mapUserToProto(*user),
	}, nil
}

func (h *UserHandler) ListUsers(
	ctx context.Context,
	req *userv1.ListUsersRequest,
) (*userv1.ListUsersResponse, error) {
	users, err := h.service.GetAll()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoUsers := make([]*userv1.User, 0, len(users))
	for _, user := range users {
		protoUsers = append(protoUsers, mapUserToProto(user))
	}

	return &userv1.ListUsersResponse{
		Users: protoUsers,
	}, nil
}

func mapUserToProto(user model.User) *userv1.User {
	return &userv1.User{
		Id:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: formatTime(user.CreatedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(time.RFC3339)
}