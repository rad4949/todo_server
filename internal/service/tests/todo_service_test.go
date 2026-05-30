package tests

import (
	"testing"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTodoService() *service.TodoService {
	repo := repository.NewPostgresTodoRepository(testDB)
	return service.NewTodoService(repo)
}

func TestTodoService_Create(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	todo, err := svc.Create("create app", nil)

	require.NoError(t, err)
	assert.Equal(t, "create app", todo.Title)
	assert.False(t, todo.Completed)
	assert.NotEmpty(t, todo.ID)
}

func TestTodoService_GetByID(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	created, err := svc.Create("find me", nil)
	require.NoError(t, err)

	found, err := svc.GetByID(created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "find me", found.Title)
}

func TestTodoService_GetByID_NotFound(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	_, err := svc.GetByID("not_created_id")

	require.Error(t, err)
}

func TestTodoService_GetAll(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	svc.Create("first", nil)
	svc.Create("second", nil)
	svc.Create("third", nil)

	todos := svc.GetAll()

	assert.Len(t, todos, 3)
}

func TestTodoService_Update(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	created, err := svc.Create("old title", nil)
	require.NoError(t, err)

	updated, err := svc.Update(created.ID, "new title", true)

	require.NoError(t, err)
	assert.Equal(t, "new title", updated.Title)
	assert.True(t, updated.Completed)
}

func TestTodoService_Delete(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	created, err := svc.Create("delete me", nil)
	require.NoError(t, err)

	err = svc.Delete(created.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(created.ID)
	assert.Error(t, err)
}

func TestTodoService_Delete_NotFound(t *testing.T) {
	cleanDB(t)
	svc := setupTodoService()

	err := svc.Delete("not_created_id")

	assert.Error(t, err)
}