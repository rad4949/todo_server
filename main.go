package main

import (
	"fmt"
	"net/http"
	"todo_server/handlers"
	"todo_server/repositories"
	"todo_server/services"
)

func main() {
	todoRepo := repositories.NewInMemoryTodoRepository()
	todoService := services.NewTodoService(todoRepo)
	todoHandler := handlers.NewTodoHandler(todoService)

	http.HandleFunc("/", todoHandler.Hello)
	http.HandleFunc("/todos", todoHandler.Todos)
	http.HandleFunc("/todos/", todoHandler.TodoByID)

	fmt.Println("Server started on :8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
