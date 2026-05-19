package model

type Todo struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Completed bool    `json:"completed"`
	UserID    *string `json:"user_id,omitempty"` // pointer бо необов'язкове
}