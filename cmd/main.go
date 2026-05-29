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
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "todo_server/docs"
	"todo_server/internal/cache"
	"todo_server/internal/config"
	"todo_server/internal/handler"
	"todo_server/internal/middleware"
	"todo_server/internal/model"
	"todo_server/internal/repository"
	"todo_server/internal/service"
	"todo_server/internal/token"
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

	redisClient := redis.NewClient(&redis.Options{
    Addr: cfg.RedisHost + ":" + cfg.RedisPort,
	})

	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		panic(fmt.Errorf("failed to connect to Redis: %w", err))
	}
	defer redisClient.Close()

	fmt.Println("Connected to Redis")

	redisCache := cache.NewRedisCache(redisClient, 7*24*time.Hour)
	blocklist := token.NewBlocklist(redisCache)

	todoRepo := repository.NewPostgresTodoRepository(db)
	userRepo := repository.NewPostgresUserRepository(db) 

	todoItemCache := cache.NewInMemoryCache[string, model.Todo]()
	todoListCache := cache.NewInMemoryCache[string, []model.Todo]()
	cachedTodoRepo := repository.NewCachedTodoRepository(todoRepo, todoItemCache, todoListCache)

	todoService := service.NewTodoService(cachedTodoRepo)
	userService := service.NewUserService(userRepo)                       
	jwtService := service.NewJWTService(cfg.JWTSecret, cfg.JWTRefreshSecret)

	todoHandler := handler.NewTodoHandler(todoService)
	userHandler := handler.NewUserHandler(userService)                    
	authHandler := handler.NewAuthHandler(jwtService, userService, blocklist)        

	rateLimiter := middleware.RateLimitMiddleware(redisCache)
	idempotency := middleware.IdempotencyMiddleware(redisCache)

	mux := http.NewServeMux()

	mux.HandleFunc("/", todoHandler.Hello)

	mux.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			todoHandler.GetTodos(w, r)
		case http.MethodPost:
			idempotency(http.HandlerFunc(todoHandler.CreateTodo)).ServeHTTP(w, r)
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

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.GetUsers(w, r)
		case http.MethodPost:
			userHandler.RegisterUser(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed"}`))
		}
	})

	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.GetUserByID(w, r)
		case http.MethodPut:
			userHandler.UpdateUser(w, r)
		case http.MethodDelete:
			userHandler.DeleteUser(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed"}`))
		}
	})

	mux.Handle("/auth/login", rateLimiter(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/logout", authHandler.Logout)

	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	handlerWithMiddleware := middleware.RecoveryMiddleware(
		middleware.CORSMiddleware(
			middleware.AuthMiddleware(jwtService)(mux),
		),
	)

	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: handlerWithMiddleware,
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