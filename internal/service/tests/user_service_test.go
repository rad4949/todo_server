package tests

import (
	"testing"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserService() *service.UserService {
	repo := repository.NewPostgresUserRepository(testDB)
	return service.NewUserService(repo)
}

func TestUserService_Register(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	user, err := svc.Register("igor", "igor@test.com", "1234")

	require.NoError(t, err)
	assert.Equal(t, "igor", user.Username)
	assert.Equal(t, "igor@test.com", user.Email)
	assert.NotEmpty(t, user.ID)
}

func TestUserService_Register_DuplicateUsername(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	_, err := svc.Register("igor", "igor@test.com", "1234")
	require.NoError(t, err)

	_, err = svc.Register("igor", "igor2@test.com", "1234")
	assert.Error(t, err)
}

func TestUserService_Authenticate_Success(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	_, err := svc.Register("igor", "igor@test.com", "1234")
	require.NoError(t, err)

	user, err := svc.Authenticate("igor", "1234")

	require.NoError(t, err)
	assert.Equal(t, "igor", user.Username)
}

func TestUserService_Authenticate_WrongPassword(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	_, err := svc.Register("igor", "igor@test.com", "1234")
	require.NoError(t, err)

	_, err = svc.Authenticate("igor", "невірний пароль")

	assert.Error(t, err)
}

func TestUserService_Authenticate_UserNotFound(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	_, err := svc.Authenticate("неіснуючий", "1234")

	assert.Error(t, err)
}

func TestUserService_GetByID(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	created, err := svc.Register("igor", "igor@test.com", "1234")
	require.NoError(t, err)

	found, err := svc.GetByID(created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "igor", found.Username)
}

func TestUserService_Update(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	created, err := svc.Register("igor", "igor@test.com", "1234")
	require.NoError(t, err)

	updated, err := svc.Update(created.ID, "igor_new", "new@test.com")

	require.NoError(t, err)
	assert.Equal(t, "igor_new", updated.Username)
	assert.Equal(t, "new@test.com", updated.Email)
}

func TestUserService_Delete(t *testing.T) {
	cleanDB(t)
	svc := setupUserService()

	created, err := svc.Register("igor", "igor@test.com", "1234")
	require.NoError(t, err)

	err = svc.Delete(created.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(created.ID)
	assert.Error(t, err)
}