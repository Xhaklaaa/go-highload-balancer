package limiter

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

func Middleware(limiter RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := getClientID(r)
			ctx := context.Background()
			if !limiter.Allow(ctx, clientID) {
				respondRateLimitExceeded(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getClientID(r *http.Request) string {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey
	}
	return r.RemoteAddr
}

func respondRateLimitExceeded(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    429,
		"message": "Rate limit exceeded",
	})
}

func ValidateContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Invalid Content-Type", http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func HandlePanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Recovered from panic: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
