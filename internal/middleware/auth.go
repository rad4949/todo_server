package middleware

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"
    "todo_server/internal/service"
)

type contextKey string
const UserIDKey contextKey = "userID"

func AuthMiddleware(jwtService *service.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if isPublicPath(r) { // ← передаємо весь r
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				writeUnauthorized(w, "invalid Authorization format, use: Bearer <token>")
				return
			}

			claims, err := jwtService.ValidateAccessToken(tokenStr)
			if err != nil {
				writeUnauthorized(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isPublicPath(r *http.Request) bool {
	path := r.URL.Path
	method := r.Method

	fullyPublic := []string{"/", "/auth/login", "/auth/refresh", "/auth/logout"}
	for _, p := range fullyPublic {
		if path == p {
			return true
		}
	}

	if path == "/users" && method == http.MethodPost {
		return true
	}

	return strings.HasPrefix(path, "/swagger")
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusUnauthorized)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}