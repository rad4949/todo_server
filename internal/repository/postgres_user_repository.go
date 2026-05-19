package repository

import (
	"database/sql"
	"fmt"
	"todo_server/internal/model"
)

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(id, username, email, hashedPassword string) (model.User, error) {
	query := `
		INSERT INTO users (id, username, email, password, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, username, email, created_at
	`

	var user model.User
	err := r.db.QueryRow(query, id, username, email, hashedPassword).
		Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (r *PostgresUserRepository) GetAll() ([]model.User, error) {
	query := `
		SELECT id, username, email, created_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get all users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *PostgresUserRepository) GetByID(id string) (*model.User, error) {
	query := `
		SELECT id, username, email, created_at
		FROM users
		WHERE id = $1
	`

	var user model.User
	err := r.db.QueryRow(query, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return &user, nil
}

func (r *PostgresUserRepository) GetByUsername(username string) (*model.User, error) {
	query := `
		SELECT id, username, email, password, created_at
		FROM users
		WHERE username = $1
	`

	// Тут SELECT включає password — потрібен для перевірки bcrypt при логіні
	var user model.User
	err := r.db.QueryRow(query, username).
		Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	return &user, nil
}

func (r *PostgresUserRepository) Update(id, username, email string) (*model.User, error) {
	query := `
		UPDATE users
		SET username = $1, email = $2
		WHERE id = $3
		RETURNING id, username, email, created_at
	`

	var user model.User
	err := r.db.QueryRow(query, username, email, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &user, nil
}

func (r *PostgresUserRepository) Delete(id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}