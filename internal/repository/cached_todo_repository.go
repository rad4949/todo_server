package repository

import (
	"todo_server/internal/cache"
	"todo_server/internal/model"
)

type CachedTodoRepository struct {
	repo      TodoRepository
	itemCache cache.Cache[string, model.Todo]   
	listCache cache.Cache[string, []model.Todo]
}

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

func (r *CachedTodoRepository) GetAll() []model.Todo {
	if cached, ok := r.listCache.Get(allTodosKey); ok {
		return cached
	}

	todos := r.repo.GetAll()
	r.listCache.Set(allTodosKey, todos)

	return todos
}

func (r *CachedTodoRepository) GetByID(id string) (*model.Todo, error) {
	if cached, ok := r.itemCache.Get(id); ok {
		return &cached, nil
	}

	todo, err := r.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	r.itemCache.Set(id, *todo)

	return todo, nil
}

func (r *CachedTodoRepository) Create(title string, userID *string) (model.Todo, error) {
	todo, err := r.repo.Create(title, userID)
	if err != nil {
		return model.Todo{}, err
	}

	r.itemCache.Set(todo.ID, todo)
	r.listCache.Delete(allTodosKey)

	return todo, nil
}

func (r *CachedTodoRepository) Update(id string, title string, completed bool) (*model.Todo, error) {
	todo, err := r.repo.Update(id, title, completed)
	if err != nil {
		return nil, err
	}

	r.itemCache.Set(id, *todo)
	r.listCache.Delete(allTodosKey)

	return todo, nil
}

func (r *CachedTodoRepository) Delete(id string) error {
	err := r.repo.Delete(id)
	if err != nil {
		return err
	}

	r.itemCache.Delete(id)
	r.listCache.Delete(allTodosKey)

	return nil
}