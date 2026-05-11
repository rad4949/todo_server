package middleware

import (
    "encoding/json"
    "fmt"
    "net/http"
    "runtime/debug"
)

func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        defer func() {
            if err := recover(); err != nil {
                fmt.Printf("[PANIC RECOVERED] %v\n%s\n", err, debug.Stack())

                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusInternalServerError) // 500
                json.NewEncoder(w).Encode(map[string]string{
                    "error": "internal server error",
                })
            }
        }()

        next.ServeHTTP(w, r)
    })
}