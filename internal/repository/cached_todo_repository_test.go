package repository

import (
	"context"
	"errors"
	"testing"

	"todo_server/internal/cache"
	"todo_server/internal/model"
)

type stubTodoRepository struct {
	createFunc func(context.Context, string, *string) (model.Todo, error)
	updateFunc func(context.Context, string, string, bool) (*model.Todo, error)
	deleteFunc func(context.Context, string) (*model.Todo, error)
}

func (s *stubTodoRepository) GetAll(context.Context) ([]model.Todo, error) {
	return nil, nil
}

func (s *stubTodoRepository) GetByID(context.Context, string) (*model.Todo, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTodoRepository) Create(ctx context.Context, title string, userID *string) (model.Todo, error) {
	return s.createFunc(ctx, title, userID)
}

func (s *stubTodoRepository) Update(ctx context.Context, id, title string, completed bool) (*model.Todo, error) {
	return s.updateFunc(ctx, id, title, completed)
}

func (s *stubTodoRepository) Delete(ctx context.Context, id string) (*model.Todo, error) {
	return s.deleteFunc(ctx, id)
}

func TestCachedTodoRepositoryCreateUpdatesCacheAfterSuccess(t *testing.T) {
	itemCache := cache.NewInMemoryCache[string, model.Todo]()
	listCache := cache.NewInMemoryCache[string, []model.Todo]()
	listCache.Set(allTodosKey, []model.Todo{{ID: "stale"}})

	repo := NewCachedTodoRepository(&stubTodoRepository{
		createFunc: func(context.Context, string, *string) (model.Todo, error) {
			return model.Todo{ID: "todo-1", Title: "created"}, nil
		},
	}, itemCache, listCache)

	todo, err := repo.Create(context.Background(), "created", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if cached, ok := itemCache.Get(todo.ID); !ok || cached.Title != todo.Title {
		t.Fatalf("item cache was not updated: cached=%+v ok=%v", cached, ok)
	}
	if _, ok := listCache.Get(allTodosKey); ok {
		t.Fatal("list cache was not invalidated")
	}
}

func TestCachedTodoRepositoryCreateDoesNotChangeCacheOnError(t *testing.T) {
	itemCache := cache.NewInMemoryCache[string, model.Todo]()
	listCache := cache.NewInMemoryCache[string, []model.Todo]()
	listCache.Set(allTodosKey, []model.Todo{{ID: "existing"}})

	repo := NewCachedTodoRepository(&stubTodoRepository{
		createFunc: func(context.Context, string, *string) (model.Todo, error) {
			return model.Todo{}, errors.New("transaction rolled back")
		},
	}, itemCache, listCache)

	if _, err := repo.Create(context.Background(), "failed", nil); err == nil {
		t.Fatal("Create() error = nil, want error")
	}
	if _, ok := listCache.Get(allTodosKey); !ok {
		t.Fatal("list cache changed after repository error")
	}
}

func TestCachedTodoRepositoryUpdateUpdatesCacheAfterSuccess(t *testing.T) {
	itemCache := cache.NewInMemoryCache[string, model.Todo]()
	listCache := cache.NewInMemoryCache[string, []model.Todo]()
	listCache.Set(allTodosKey, []model.Todo{{ID: "todo-1", Title: "old"}})

	repo := NewCachedTodoRepository(&stubTodoRepository{
		updateFunc: func(context.Context, string, string, bool) (*model.Todo, error) {
			return &model.Todo{ID: "todo-1", Title: "updated", Completed: true}, nil
		},
	}, itemCache, listCache)

	updated, err := repo.Update(context.Background(), "todo-1", "updated", true)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if cached, ok := itemCache.Get(updated.ID); !ok || cached.Title != "updated" || !cached.Completed {
		t.Fatalf("item cache was not updated: cached=%+v ok=%v", cached, ok)
	}
	if _, ok := listCache.Get(allTodosKey); ok {
		t.Fatal("list cache was not invalidated")
	}
}

func TestCachedTodoRepositoryUpdateDoesNotChangeCacheOnError(t *testing.T) {
	itemCache := cache.NewInMemoryCache[string, model.Todo]()
	listCache := cache.NewInMemoryCache[string, []model.Todo]()
	itemCache.Set("todo-1", model.Todo{ID: "todo-1", Title: "old"})
	listCache.Set(allTodosKey, []model.Todo{{ID: "todo-1", Title: "old"}})

	repo := NewCachedTodoRepository(&stubTodoRepository{
		updateFunc: func(context.Context, string, string, bool) (*model.Todo, error) {
			return nil, errors.New("transaction rolled back")
		},
	}, itemCache, listCache)

	if _, err := repo.Update(context.Background(), "todo-1", "new", true); err == nil {
		t.Fatal("Update() error = nil, want error")
	}
	if cached, ok := itemCache.Get("todo-1"); !ok || cached.Title != "old" {
		t.Fatalf("item cache changed after repository error: cached=%+v ok=%v", cached, ok)
	}
	if cached, ok := listCache.Get(allTodosKey); !ok || len(cached) != 1 || cached[0].Title != "old" {
		t.Fatalf("list cache changed after repository error: cached=%+v ok=%v", cached, ok)
	}
}

func TestCachedTodoRepositoryDeleteInvalidatesCacheAfterSuccess(t *testing.T) {
	itemCache := cache.NewInMemoryCache[string, model.Todo]()
	listCache := cache.NewInMemoryCache[string, []model.Todo]()
	itemCache.Set("todo-1", model.Todo{ID: "todo-1"})
	listCache.Set(allTodosKey, []model.Todo{{ID: "todo-1"}})

	repo := NewCachedTodoRepository(&stubTodoRepository{
		deleteFunc: func(context.Context, string) (*model.Todo, error) {
			return &model.Todo{ID: "todo-1", Title: "deleted"}, nil
		},
	}, itemCache, listCache)

	deleted, err := repo.Delete(context.Background(), "todo-1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted.ID != "todo-1" {
		t.Fatalf("Delete() id = %q, want todo-1", deleted.ID)
	}
	if _, ok := itemCache.Get("todo-1"); ok {
		t.Fatal("item cache was not invalidated")
	}
	if _, ok := listCache.Get(allTodosKey); ok {
		t.Fatal("list cache was not invalidated")
	}
}

func TestCachedTodoRepositoryDeleteDoesNotChangeCacheOnError(t *testing.T) {
	itemCache := cache.NewInMemoryCache[string, model.Todo]()
	listCache := cache.NewInMemoryCache[string, []model.Todo]()
	itemCache.Set("todo-1", model.Todo{ID: "todo-1"})
	listCache.Set(allTodosKey, []model.Todo{{ID: "todo-1"}})

	repo := NewCachedTodoRepository(&stubTodoRepository{
		deleteFunc: func(context.Context, string) (*model.Todo, error) {
			return nil, errors.New("transaction rolled back")
		},
	}, itemCache, listCache)

	if _, err := repo.Delete(context.Background(), "todo-1"); err == nil {
		t.Fatal("Delete() error = nil, want error")
	}
	if _, ok := itemCache.Get("todo-1"); !ok {
		t.Fatal("item cache changed after repository error")
	}
	if _, ok := listCache.Get(allTodosKey); !ok {
		t.Fatal("list cache changed after repository error")
	}
}
