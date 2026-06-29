package grpchandler

import (
	"context"
	"strings"

	todov1 "todo_server/internal/gen/todo/v1"
	"todo_server/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"todo_server/internal/handler/grpc/interceptors"
)

type TodoService interface {
	Create(title string, userID *string) (model.Todo, error)
	GetByID(id string) (*model.Todo, error)
	GetAll() []model.Todo
	Update(id string, title string, completed bool) (*model.Todo, error)
	Delete(id string) error
}

type TodoHandler struct {
	todov1.UnimplementedTodoServiceServer
	service TodoService
}

func NewTodoHandler(service TodoService) *TodoHandler {
	return &TodoHandler{
		service: service,
	}
}

func (h *TodoHandler) CreateTodo(
	ctx context.Context,
	req *todov1.CreateTodoRequest,
) (*todov1.TodoResponse, error) {
	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	todo, err := h.service.Create(title, &userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &todov1.TodoResponse{
		Todo: mapTodoToProto(todo),
	}, nil
}

func (h *TodoHandler) GetTodo(
	ctx context.Context,
	req *todov1.GetTodoRequest,
) (*todov1.TodoResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	todo, err := h.service.GetByID(id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &todov1.TodoResponse{
		Todo: mapTodoToProto(*todo),
	}, nil
}

func (h *TodoHandler) ListTodos(
	ctx context.Context,
	req *todov1.ListTodosRequest,
) (*todov1.ListTodosResponse, error) {
	todos := h.service.GetAll()

	protoTodos := make([]*todov1.Todo, 0, len(todos))
	for _, todo := range todos {
		protoTodos = append(protoTodos, mapTodoToProto(todo))
	}

	return &todov1.ListTodosResponse{
		Todos: protoTodos,
	}, nil
}

func (h *TodoHandler) UpdateTodo(
	ctx context.Context,
	req *todov1.UpdateTodoRequest,
) (*todov1.TodoResponse, error) {
	id := strings.TrimSpace(req.GetId())
	title := strings.TrimSpace(req.GetTitle())

	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	todo, err := h.service.Update(id, title, req.GetCompleted())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &todov1.TodoResponse{
		Todo: mapTodoToProto(*todo),
	}, nil
}

func (h *TodoHandler) DeleteTodo(
	ctx context.Context,
	req *todov1.DeleteTodoRequest,
) (*todov1.DeleteTodoResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := h.service.Delete(id); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &todov1.DeleteTodoResponse{
		Success: true,
	}, nil
}

func (h *TodoHandler) WatchTodos(
	req *todov1.WatchTodosRequest,
	stream todov1.TodoService_WatchTodosServer,
) error {
	todos := h.service.GetAll()

	for _, todo := range todos {
		event := &todov1.TodoEvent{
			Type: "TODO_SNAPSHOT",
			Todo: mapTodoToProto(todo),
		}

		if err := stream.Send(event); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}

	return nil
}

func mapTodoToProto(todo model.Todo) *todov1.Todo {
	userID := ""
	if todo.UserID != nil {
		userID = *todo.UserID
	}

	return &todov1.Todo{
		Id:        todo.ID,
		Title:     todo.Title,
		Completed: todo.Completed,
		UserId:    userID,
	}
}
