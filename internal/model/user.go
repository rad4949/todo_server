package model

import "time"

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`         // json:"-" означає НІКОЛИ не повертати пароль клієнту
	CreatedAt time.Time `json:"created_at"`
}