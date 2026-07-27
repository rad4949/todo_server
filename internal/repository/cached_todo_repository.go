package repository

import (
	"context"
	"todo_server/internal/cache"
	"todo_server/internal/model"
)

type CachedTodoRepository struct {
	repo      TodoRepository
	itemCache cache.Cache[string, model.Todo]
	listCache cache.Cache[string, []model.Todo]
}

var _ TodoRepository = (*CachedTodoRepository)(nil)
const allTodosKey = "all"

func NewCachedTodoRepository(
	repo TodoRepository,
	itemCache cache.Cache[string, model.Todo],
	listCache cache.Cache[string, []model.Todo],
) TodoRepository {
	return &CachedTodoRepository{
		repo:      repo,
		itemCache: itemCache,
		listCache: listCache,
	}
}

func (r *CachedTodoRepository) GetAll(ctx context.Context) ([]model.Todo, error) {
	if cached, ok := r.listCache.Get(allTodosKey); ok {
		return cached, nil
	}

	todos, err := r.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	r.listCache.Set(allTodosKey, todos)

	return todos, nil
}

func (r *CachedTodoRepository) GetByID(ctx context.Context, id string) (*model.Todo, error) {
	if cached, ok := r.itemCache.Get(id); ok {
		return &cached, nil
	}

	todo, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	r.itemCache.Set(id, *todo)

	return todo, nil
}

func (r *CachedTodoRepository) Create(ctx context.Context, title string, userID *string) (model.Todo, error) {
	todo, err := r.repo.Create(ctx, title, userID)
	if err != nil {
		return model.Todo{}, err
	}

	r.itemCache.Set(todo.ID, todo)
	r.listCache.Delete(allTodosKey)

	return todo, nil
}

func (r *CachedTodoRepository) Update(ctx context.Context, id string, title string, completed bool) (*model.Todo, error) {
	todo, err := r.repo.Update(ctx, id, title, completed)
	if err != nil {
		return nil, err
	}

	r.itemCache.Set(id, *todo)
	r.listCache.Delete(allTodosKey)

	return todo, nil
}

func (r *CachedTodoRepository) Delete(ctx context.Context, id string) (*model.Todo, error) {
	todo, err := r.repo.Delete(ctx, id)
	if err != nil {
		return nil, err
	}

	r.itemCache.Delete(id)
	r.listCache.Delete(allTodosKey)

	return todo, nil
}
