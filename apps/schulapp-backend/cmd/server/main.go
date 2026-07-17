package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"schulapp/internal/api/handler"
	appmw "schulapp/internal/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL nicht gesetzt")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("DB öffnen:", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("DB Ping:", err)
	}

	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET nicht gesetzt")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	authHandler := &handler.AuthHandler{DB: db, JWTSecret: jwtSecret}

	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Login-Rate-Limiter: 5 Anfragen pro Minute
	loginLimiter := rate.NewLimiter(rate.Every(time.Minute/5), 5)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if !loginLimiter.Allow() {
						http.Error(w, `{"error":"zu viele Anfragen"}`, http.StatusTooManyRequests)
						return
					}
					next.ServeHTTP(w, req)
				})
			}).Post("/login", authHandler.Login)

			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)

			r.With(appmw.Auth(jwtSecret)).Get("/me", authHandler.Me)
			r.With(appmw.Auth(jwtSecret)).Patch("/me", authHandler.UpdateMe)
		})
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Server läuft auf http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
