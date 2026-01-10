package middleware

import (
	"github.com/go-chi/cors"
)

// CORS returns a configured CORS middleware.
func CORS() func(next interface{}) interface{} {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "X-Stream-URL"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
