package main
 
import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
 
	_ "github.com/lib/pq"
	httpSwagger "github.com/swaggo/http-swagger"
 
	"todo_server/config"
	_ "todo_server/docs"
	"todo_server/internal/handler"
	"todo_server/internal/repository"
	"todo_server/internal/service"
)
 
// @title Todo API
// @version 1.0
// @description Simple Todo API
// @host localhost:8080
// @BasePath /
func main() {
 
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
 
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

	mux := http.NewServeMux()
 
	mux.HandleFunc("/", todoHandler.Hello)
 
	mux.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
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
 
	mux.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
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
 
	mux.Handle("/swagger/", httpSwagger.WrapHandler)
 
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: mux,
	}
 
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
 
	go func() {
		fmt.Println("Server started on :" + cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
 
	<-quit
	fmt.Println("Shutting down server...")
 
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
 
	if err := server.Shutdown(ctx); err != nil {
		fmt.Println("Server forced to shutdown:", err)
	}
 
	fmt.Println("Server stopped gracefully")
}
