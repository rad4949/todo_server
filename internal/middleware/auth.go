package middleware

import (
    "encoding/json"
    "net/http"
)

const validToken = "super-secret-token-igor-2024"

func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        if r.URL.Path == "/" || isSwaggerPath(r.URL.Path) {
            next.ServeHTTP(w, r)
            return
        }

        token := r.Header.Get("X-Auth-Token")

        if token == "" {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized) // 401
            json.NewEncoder(w).Encode(map[string]string{
                "error": "missing X-Auth-Token header",
            })
            return 
        }

        if token != validToken {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized) // 401
            json.NewEncoder(w).Encode(map[string]string{
                "error": "invalid token",
            })
            return
        }

        next.ServeHTTP(w, r)
    })
}

func isSwaggerPath(path string) bool {
    return len(path) >= 8 && path[:8] == "/swagger"
}