package grpchandler

import (
	"context"
	"io"
	"strings"

	todov1 "todo_server/internal/gen/todo/v1"
	"todo_server/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"todo_server/internal/handler/grpc/interceptors"
)

type TodoService interface {
	Create(ctx context.Context, title string, userID *string) (model.Todo, error)
	GetByID(ctx context.Context, id string) (*model.Todo, error)
	GetAll(ctx context.Context) ([]model.Todo, error)
	Update(ctx context.Context, id string, title string, completed bool) (*model.Todo, error)
	Delete(ctx context.Context, id string) (*model.Todo, error)
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

	todo, err := h.service.Create(ctx, title, &userID)
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

	todo, err := h.service.GetByID(ctx, id)
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
	todos, err := h.service.GetAll(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

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

	todo, err := h.service.Update(ctx, id, title, req.GetCompleted())
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

	if _, err := h.service.Delete(ctx, id); err != nil {
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
	todos, err := h.service.GetAll(stream.Context())
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

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

func (h *TodoHandler) BulkCreateTodos(
	stream todov1.TodoService_BulkCreateTodosServer,
) error {
	ctx := stream.Context()
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	var todos []*todov1.Todo

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&todov1.BulkCreateTodosResponse{
				CreatedCount: int32(len(todos)),
				Todos:        todos,
			})
		}

		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		title := strings.TrimSpace(req.GetTitle())
		if title == "" {
			return status.Error(codes.InvalidArgument, "title is required")
		}

		todo, err := h.service.Create(stream.Context(), title, &userID)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		todos = append(todos, mapTodoToProto(todo))
	}
}

func (h *TodoHandler) SyncTodos(
	stream todov1.TodoService_SyncTodosServer,
) error {
	ctx := stream.Context()
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		action := strings.ToUpper(strings.TrimSpace(req.GetAction()))

		switch action {
		case "CREATE":
			title := strings.TrimSpace(req.GetTitle())
			if title == "" {
				if err := stream.Send(&todov1.TodoSyncEvent{
					Type:  "ERROR",
					Error: "title is required",
				}); err != nil {
					return status.Error(codes.Internal, err.Error())
				}
				continue
			}

			todo, err := h.service.Create(stream.Context(), title, &userID)
			if err != nil {
				if err := stream.Send(&todov1.TodoSyncEvent{
					Type:  "ERROR",
					Error: err.Error(),
				}); err != nil {
					return status.Error(codes.Internal, err.Error())
				}
				continue
			}

			if err := stream.Send(&todov1.TodoSyncEvent{
				Type: "CREATED",
				Todo: mapTodoToProto(todo),
			}); err != nil {
				return status.Error(codes.Internal, err.Error())
			}

		default:
			if err := stream.Send(&todov1.TodoSyncEvent{
				Type:  "ERROR",
				Error: "unsupported action",
			}); err != nil {
				return status.Error(codes.Internal, err.Error())
			}
		}
	}
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
