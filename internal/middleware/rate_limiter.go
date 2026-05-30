package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"todo_server/internal/cache"
)

const (
	maxAttempts = 5
	windowTime  = 1 * time.Minute
)

func RateLimitMiddleware(redisCache *cache.RedisCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			key := "rate_limit:login:" + ip

			attempts, err := redisCache.Increment(context.Background(), key, windowTime)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if attempts > maxAttempts {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "too many login attempts, try again in 1 minute",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}