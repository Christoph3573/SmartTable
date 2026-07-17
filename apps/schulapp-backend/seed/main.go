package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL nicht gesetzt")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Datenbankverbindung fehlgeschlagen:", err)
	}

	hash := func(pw string) string {
		h, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
		if err != nil {
			log.Fatal(err)
		}
		return string(h)
	}

	users := []struct {
		email, pw, first, last, role string
	}{
		{"admin@schule.de", "admin123", "Admin", "Schule", "admin"},
		{"mueller@schule.de", "lehrer123", "Hans", "Müller", "teacher"},
		{"schmidt@schule.de", "lehrer123", "Anna", "Schmidt", "teacher"},
		{"weber@schule.de", "lehrer123", "Klaus", "Weber", "teacher"},
	}
	for i := 1; i <= 10; i++ {
		users = append(users, struct {
			email, pw, first, last, role string
		}{
			fmt.Sprintf("schueler%d@schule.de", i),
			"schueler123",
			fmt.Sprintf("Schüler%d", i),
			"Mustermann",
			"student",
		})
	}

	for _, u := range users {
		_, err := db.Exec(
			`INSERT INTO users (email, password_hash, first_name, last_name, role)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (email) DO NOTHING`,
			u.email, hash(u.pw), u.first, u.last, u.role,
		)
		if err != nil {
			log.Printf("Fehler bei %s: %v", u.email, err)
		} else {
			fmt.Printf("✓ %s (%s)\n", u.email, u.role)
		}
	}

	// Klasse anlegen
	var classID int
	err = db.QueryRow(
		`INSERT INTO classes (name, school_year) VALUES ('10a', '2025/26')
		 ON CONFLICT DO NOTHING RETURNING id`,
	).Scan(&classID)
	if err != nil && err != sql.ErrNoRows {
		log.Println("Klasse:", err)
	}

	fmt.Println("Seed abgeschlossen.")
}
