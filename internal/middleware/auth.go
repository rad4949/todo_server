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

            // Публічні маршрути — без токена
            if isPublicPath(r.URL.Path) {
                next.ServeHTTP(w, r)
                return
            }

            // Читаємо заголовок: "Authorization: Bearer eyJ..."
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                writeUnauthorized(w, "missing Authorization header")
                return
            }

            // Відрізаємо "Bearer " — залишається сам токен
            tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenStr == authHeader { // TrimPrefix нічого не відрізав
                writeUnauthorized(w, "invalid Authorization format, use: Bearer <token>")
                return
            }

            // Валідуємо токен
            claims, err := jwtService.ValidateAccessToken(tokenStr)
            if err != nil {
                writeUnauthorized(w, "invalid or expired token")
                return
            }

            // Передаємо userID далі через context
            ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func isPublicPath(path string) bool {
    public := []string{"/", "/auth/login", "/auth/refresh"}
    for _, p := range public {
        if path == p {
            return true
        }
    }
    return strings.HasPrefix(path, "/swagger")
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusUnauthorized)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}