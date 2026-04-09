package main

import (
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
	httpSwagger "github.com/swaggo/http-swagger"

	"todo_server/config"
	_ "todo_server/docs"
	"todo_server/handler"
	"todo_server/repository"
	"todo_server/service"
)

// @title Todo API
// @version 1.0
// @description Simple Todo API
// @host localhost:8080
// @BasePath /
func main() {

	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}
	fmt.Println(cfg.DBHost)
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
		panic(fmt.Errorf("failed to open DB: %w", err))
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(fmt.Errorf("failed to ping DB: %w", err))
	}

	fmt.Println("Connected to PostgreSQL")

	todoRepo := repository.NewPostgresTodoRepository(db)
	todoService := service.NewTodoService(todoRepo)
	todoHandler := handler.NewTodoHandler(todoService)

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
		panic(fmt.Errorf("server failed: %w", err))
	}
}
