package middleware

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"todo_server/internal/cache"
)

const idempotencyTTL = 24 * time.Hour

func IdempotencyMiddleware(redisCache *cache.RedisCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idempotencyKey := r.Header.Get("Idempotency-Key")
			if idempotencyKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			key := "idempotency:" + idempotencyKey

			if cachedResponse, found := redisCache.Get(context.Background(), key); found {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(cachedResponse))
				return
			}

			recorder := &responseRecorder{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			if recorder.statusCode >= 200 && recorder.statusCode < 300 {
				redisCache.SetWithTTL(
					context.Background(),
					key,
					recorder.body.String(),
					idempotencyTTL,
				)
			}
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}