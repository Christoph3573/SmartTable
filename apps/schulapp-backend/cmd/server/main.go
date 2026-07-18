package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"schulapp/internal/api"
	"schulapp/internal/api/handler"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
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

	seedAdminUser(db)

	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET nicht gesetzt")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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

	srv := &handler.Server{
		DB:           db,
		JWTSecret:    jwtSecret,
		LoginLimiter: rate.NewLimiter(rate.Every(time.Minute/5), 5),
	}

	api.HandlerWithOptions(srv, api.ChiServerOptions{
		BaseRouter: r,
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Server läuft auf http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func seedAdminUser(db *sql.DB) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = 'admin@schule.de')`).Scan(&exists)
	if err != nil {
		log.Printf("seed: Fehler beim Prüfen auf Admin-User: %v", err)
		return
	}
	if exists {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		log.Printf("seed: Fehler beim Hash-Generieren: %v", err)
		return
	}

	_, err = db.Exec(
		`INSERT INTO users (email, password_hash, first_name, last_name, role) VALUES ($1, $2, $3, $4, $5)`,
		"admin@schule.de", string(hash), "Admin", "Schule", "admin",
	)
	if err != nil {
		log.Printf("seed: Fehler beim Anlegen des Admin-Users: %v", err)
		return
	}
	log.Println("seed: Admin-User angelegt (admin@schule.de / admin123)")
}
