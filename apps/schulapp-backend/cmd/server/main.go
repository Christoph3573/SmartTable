package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"schulapp/internal/api"
	"schulapp/internal/api/handler"
	appmw "schulapp/internal/middleware"
	"schulapp/internal/ws"

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

	if os.Getenv("DEV_SEED_ADMIN") == "true" {
		seedAdminUser(db)
	}

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
		AllowedOrigins:   corsOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	// The OpenAPI router registers public and protected routes together. Keep the
	// small public surface explicit and authenticate every other API operation.
	r.Use(func(next http.Handler) http.Handler {
		protected := appmw.Auth(jwtSecret)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/health" || req.URL.Path == "/api/v1/health" || req.URL.Path == "/ws" ||
				req.URL.Path == "/api/v1/auth/login" || req.URL.Path == "/api/v1/auth/refresh" || req.URL.Path == "/api/v1/auth/logout" {
				next.ServeHTTP(w, req)
				return
			}
			protected.ServeHTTP(w, req)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &handler.Server{
		DB:           db,
		JWTSecret:    jwtSecret,
		LoginLimiter: rate.NewLimiter(rate.Every(time.Minute/5), 5),
		// UPLOAD_DIR is the deployed Compose setting. FILE_STORAGE_PATH remains a
		// backwards-compatible local override.
		UploadDir: envOrDefault("UPLOAD_DIR", envOrDefault("FILE_STORAGE_PATH", "./data/uploads")),
		Hub:       ws.NewHub(),
	}

	r.Get("/ws", srv.HandleWS)

	api.HandlerWithOptions(srv, api.ChiServerOptions{
		BaseRouter: r,
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Server läuft auf http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// corsOrigins reads a comma-separated CORS_ORIGINS override (needed for LAN/Tailscale
// access during development); falls back to the default local Vite/CRA dev ports.
func corsOrigins() []string {
	raw := os.Getenv("CORS_ORIGINS")
	if raw == "" {
		return []string{"http://localhost:5173", "http://localhost:3000"}
	}
	origins := strings.Split(raw, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}
	return origins
}

func seedAdminUser(db *sql.DB) {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		log.Printf("seed: Fehler beim Hash-Generieren: %v", err)
		return
	}

	_, err = db.Exec(
		`INSERT INTO users (email, password_hash, first_name, last_name, role, active)
		 VALUES ($1, $2, $3, $4, $5, true)
		 ON CONFLICT (email) DO UPDATE SET
		   password_hash = EXCLUDED.password_hash,
		   role = 'admin',
		   active = true`,
		"admin@schule.de", string(hash), "Admin", "Schule", "admin",
	)
	if err != nil {
		log.Printf("seed: Fehler beim Anlegen/Updaten des Admin-Users: %v", err)
		return
	}
	log.Println("seed: Entwicklungs-Admin sichergestellt")
}
