package main

import (
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
	httpSwagger "github.com/swaggo/http-swagger"

	"todo_server/config"
	_ "todo_server/docs"
	"todo_server/handlers"
	"todo_server/repositories"
	"todo_server/services"
)

// @title Todo API
// @version 1.0
// @description Simple Todo API
// @host localhost:8080
// @BasePath /
func main() {

	cfg := config.Load()
	addr := ":" + cfg.ServerPort
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("Connected to PostgreSQL")

	todoRepo := repositories.NewInMemoryTodoRepository()
	todoService := services.NewTodoService(todoRepo)
	todoHandler := handlers.NewTodoHandler(todoService)

	http.HandleFunc("/", todoHandler.Hello)

	http.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			todoHandler.GetTodos(w, r)
		case http.MethodPost:
			todoHandler.CreateTodo(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed"}`))
		}
	})

	http.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			todoHandler.GetTodoByID(w, r)
		case http.MethodPut:
			todoHandler.UpdateTodo(w, r)
		case http.MethodDelete:
			todoHandler.DeleteTodo(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed"}`))
		}
	})

	http.Handle("/swagger/", httpSwagger.WrapHandler)

	fmt.Println("Server started on", addr)
	err = http.ListenAndServe(addr, nil)

	if err != nil {
		panic(err)
	}
}
