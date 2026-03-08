package main

import (
	"fmt"
	"net/http"
	"todo_server/handlers"
	"todo_server/services"
)

func main() {
	todoService := services.NewTodoService()
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
