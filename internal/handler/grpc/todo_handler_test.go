package grpchandler

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	todov1 "todo_server/internal/gen/todo/v1"
	"todo_server/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/test/bufconn"
	"todo_server/internal/handler/grpc/interceptors"
)

type fakeTodoService struct {
	createFunc  func(title string, userID *string) (model.Todo, error)
	getByIDFunc func(id string) (*model.Todo, error)
	getAllFunc  func() []model.Todo
	updateFunc  func(id string, title string, completed bool) (*model.Todo, error)
	deleteFunc  func(id string) error
}

func (f *fakeTodoService) Create(_ context.Context, title string, userID *string) (model.Todo, error) {
	if f.createFunc != nil {
		return f.createFunc(title, userID)
	}

	return model.Todo{}, nil
}

func (f *fakeTodoService) GetByID(_ context.Context, id string) (*model.Todo, error) {
	if f.getByIDFunc != nil {
		return f.getByIDFunc(id)
	}

	return nil, errors.New("not found")
}

func (f *fakeTodoService) GetAll(_ context.Context) ([]model.Todo, error) {
	if f.getAllFunc != nil {
		return f.getAllFunc(), nil
	}

	return []model.Todo{}, nil
}

func (f *fakeTodoService) Update(_ context.Context, id string, title string, completed bool) (*model.Todo, error) {
	if f.updateFunc != nil {
		return f.updateFunc(id, title, completed)
	}

	return nil, errors.New("not found")
}

func (f *fakeTodoService) Delete(_ context.Context, id string) (*model.Todo, error) {
	if f.deleteFunc != nil {
		if err := f.deleteFunc(id); err != nil {
			return nil, err
		}
	}

	return &model.Todo{ID: id}, nil
}

type testServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *testServerStream) Context() context.Context {
	return s.ctx
}

func newTodoTestClient(t *testing.T, todoService TodoService) todov1.TodoServiceClient {
	t.Helper()

	listener := bufconn.Listen(bufSize)

	server := grpc.NewServer(
		grpc.UnaryInterceptor(func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (any, error) {
			ctx = context.WithValue(ctx, interceptors.UserIDKey, "user-1")
			ctx = context.WithValue(ctx, interceptors.UsernameKey, "grpc_user")
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(
			srv any,
			stream grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
			ctx := context.WithValue(stream.Context(), interceptors.UserIDKey, "user-1")
			ctx = context.WithValue(ctx, interceptors.UsernameKey, "grpc_user")

			wrappedStream := &testServerStream{
				ServerStream: stream,
				ctx:          ctx,
			}

			return handler(srv, wrappedStream)
		}),
	)
	todov1.RegisterTodoServiceServer(server, NewTodoHandler(todoService))

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

	return todov1.NewTodoServiceClient(conn)
}

func TestTodoHandler_CreateTodo_Success(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		createFunc: func(title string, userID *string) (model.Todo, error) {
			if title != "Learn gRPC" {
				t.Fatalf("expected title Learn gRPC, got %s", title)
			}

			if userID == nil {
				t.Fatalf("expected userID, got nil")
			}

			if *userID != "user-1" {
				t.Fatalf("expected userID user-1, got %s", *userID)
			}

			return model.Todo{
				ID:        "todo-1",
				Title:     title,
				Completed: false,
			}, nil
		},
	})

	resp, err := client.CreateTodo(context.Background(), &todov1.CreateTodoRequest{
		Title: "Learn gRPC",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetTodo().GetId() != "todo-1" {
		t.Fatalf("expected todo id todo-1, got %s", resp.GetTodo().GetId())
	}

	if resp.GetTodo().GetTitle() != "Learn gRPC" {
		t.Fatalf("expected title Learn gRPC, got %s", resp.GetTodo().GetTitle())
	}
}

func TestTodoHandler_CreateTodo_EmptyTitle(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{})

	_, err := client.CreateTodo(context.Background(), &todov1.CreateTodoRequest{
		Title: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestTodoHandler_GetTodo_Success(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		getByIDFunc: func(id string) (*model.Todo, error) {
			if id != "todo-1" {
				t.Fatalf("expected id todo-1, got %s", id)
			}

			return &model.Todo{
				ID:        "todo-1",
				Title:     "Learn gRPC",
				Completed: false,
			}, nil
		},
	})

	resp, err := client.GetTodo(context.Background(), &todov1.GetTodoRequest{
		Id: "todo-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetTodo().GetId() != "todo-1" {
		t.Fatalf("expected todo id todo-1, got %s", resp.GetTodo().GetId())
	}
}

func TestTodoHandler_GetTodo_EmptyID(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{})

	_, err := client.GetTodo(context.Background(), &todov1.GetTodoRequest{
		Id: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestTodoHandler_GetTodo_NotFound(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		getByIDFunc: func(id string) (*model.Todo, error) {
			return nil, errors.New("todo not found")
		},
	})

	_, err := client.GetTodo(context.Background(), &todov1.GetTodoRequest{
		Id: "missing-id",
	})

	assertGRPCCode(t, err, codes.NotFound)
}

func TestTodoHandler_ListTodos_Success(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		getAllFunc: func() []model.Todo {
			return []model.Todo{
				{
					ID:        "todo-1",
					Title:     "Learn gRPC",
					Completed: false,
				},
				{
					ID:        "todo-2",
					Title:     "Write tests",
					Completed: true,
				},
			}
		},
	})

	resp, err := client.ListTodos(context.Background(), &todov1.ListTodosRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.GetTodos()) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(resp.GetTodos()))
	}
}

func TestTodoHandler_UpdateTodo_Success(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		updateFunc: func(id string, title string, completed bool) (*model.Todo, error) {
			if id != "todo-1" {
				t.Fatalf("expected id todo-1, got %s", id)
			}

			if title != "Updated title" {
				t.Fatalf("expected title Updated title, got %s", title)
			}

			if !completed {
				t.Fatalf("expected completed true, got false")
			}

			return &model.Todo{
				ID:        id,
				Title:     title,
				Completed: completed,
			}, nil
		},
	})

	resp, err := client.UpdateTodo(context.Background(), &todov1.UpdateTodoRequest{
		Id:        "todo-1",
		Title:     "Updated title",
		Completed: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.GetTodo().GetTitle() != "Updated title" {
		t.Fatalf("expected updated title, got %s", resp.GetTodo().GetTitle())
	}

	if !resp.GetTodo().GetCompleted() {
		t.Fatalf("expected completed true")
	}
}

func TestTodoHandler_UpdateTodo_EmptyID(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{})

	_, err := client.UpdateTodo(context.Background(), &todov1.UpdateTodoRequest{
		Id:        "",
		Title:     "Updated title",
		Completed: true,
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestTodoHandler_UpdateTodo_EmptyTitle(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{})

	_, err := client.UpdateTodo(context.Background(), &todov1.UpdateTodoRequest{
		Id:        "todo-1",
		Title:     "",
		Completed: true,
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestTodoHandler_UpdateTodo_NotFound(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		updateFunc: func(id string, title string, completed bool) (*model.Todo, error) {
			return nil, errors.New("todo not found")
		},
	})

	_, err := client.UpdateTodo(context.Background(), &todov1.UpdateTodoRequest{
		Id:        "missing-id",
		Title:     "Updated title",
		Completed: true,
	})

	assertGRPCCode(t, err, codes.NotFound)
}

func TestTodoHandler_DeleteTodo_Success(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		deleteFunc: func(id string) error {
			if id != "todo-1" {
				t.Fatalf("expected id todo-1, got %s", id)
			}

			return nil
		},
	})

	resp, err := client.DeleteTodo(context.Background(), &todov1.DeleteTodoRequest{
		Id: "todo-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !resp.GetSuccess() {
		t.Fatalf("expected success true")
	}
}

func TestTodoHandler_DeleteTodo_EmptyID(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{})

	_, err := client.DeleteTodo(context.Background(), &todov1.DeleteTodoRequest{
		Id: "",
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestTodoHandler_DeleteTodo_NotFound(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		deleteFunc: func(id string) error {
			return errors.New("todo not found")
		},
	})

	_, err := client.DeleteTodo(context.Background(), &todov1.DeleteTodoRequest{
		Id: "missing-id",
	})

	assertGRPCCode(t, err, codes.NotFound)
}

func TestTodoHandler_WatchTodos_Success(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		getAllFunc: func() []model.Todo {
			return []model.Todo{
				{
					ID:        "todo-1",
					Title:     "Learn server streaming",
					Completed: false,
				},
				{
					ID:        "todo-2",
					Title:     "Write stream test",
					Completed: true,
				},
			}
		},
	})

	stream, err := client.WatchTodos(context.Background(), &todov1.WatchTodosRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	firstEvent, err := stream.Recv()
	if err != nil {
		t.Fatalf("expected first event, got error: %v", err)
	}

	if firstEvent.GetType() != "TODO_SNAPSHOT" {
		t.Fatalf("expected TODO_SNAPSHOT, got %s", firstEvent.GetType())
	}

	if firstEvent.GetTodo().GetId() != "todo-1" {
		t.Fatalf("expected todo-1, got %s", firstEvent.GetTodo().GetId())
	}

	secondEvent, err := stream.Recv()
	if err != nil {
		t.Fatalf("expected second event, got error: %v", err)
	}

	if secondEvent.GetType() != "TODO_SNAPSHOT" {
		t.Fatalf("expected TODO_SNAPSHOT, got %s", secondEvent.GetType())
	}

	if secondEvent.GetTodo().GetId() != "todo-2" {
		t.Fatalf("expected todo-2, got %s", secondEvent.GetTodo().GetId())
	}

	_, err = stream.Recv()
	if err != io.EOF {
		t.Fatalf("expected EOF after all events, got %v", err)
	}
}

func TestTodoHandler_SyncTodos_CreateAndError(t *testing.T) {
	client := newTodoTestClient(t, &fakeTodoService{
		createFunc: func(title string, userID *string) (model.Todo, error) {
			if userID == nil {
				t.Fatalf("expected userID, got nil")
			}

			return model.Todo{
				ID:        "todo-" + title,
				Title:     title,
				Completed: false,
				UserID:    userID,
			}, nil
		},
	})

	stream, err := client.SyncTodos(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = stream.Send(&todov1.TodoSyncRequest{
		Action: "CREATE",
		Title:  "Bidi test todo",
	})
	if err != nil {
		t.Fatalf("expected send success, got %v", err)
	}

	event, err := stream.Recv()
	if err != nil {
		t.Fatalf("expected created event, got %v", err)
	}

	if event.GetType() != "CREATED" {
		t.Fatalf("expected CREATED, got %s", event.GetType())
	}

	if event.GetTodo().GetTitle() != "Bidi test todo" {
		t.Fatalf("expected title Bidi test todo, got %s", event.GetTodo().GetTitle())
	}

	err = stream.Send(&todov1.TodoSyncRequest{
		Action: "UNKNOWN",
		Title:  "Unsupported action",
	})
	if err != nil {
		t.Fatalf("expected send success, got %v", err)
	}

	event, err = stream.Recv()
	if err != nil {
		t.Fatalf("expected error event, got %v", err)
	}

	if event.GetType() != "ERROR" {
		t.Fatalf("expected ERROR, got %s", event.GetType())
	}

	if event.GetError() != "unsupported action" {
		t.Fatalf("expected unsupported action error, got %s", event.GetError())
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("expected close send success, got %v", err)
	}
}
