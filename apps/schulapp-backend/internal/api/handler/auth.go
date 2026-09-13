package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"schulapp/internal/api"
	appmw "schulapp/internal/middleware"
	"schulapp/internal/ws"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type Server struct {
	api.Unimplemented

	DB           *sql.DB
	JWTSecret    []byte
	LoginLimiter *rate.Limiter
	UploadDir    string
	Hub          *ws.Hub
}

func (h *Server) PostApiV1AuthLogin(w http.ResponseWriter, r *http.Request) {
	if !h.LoginLimiter.Allow() {
		writeError(w, http.StatusTooManyRequests, "zu viele Anfragen")
		return
	}

	var req api.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültige Anfrage")
		return
	}

	var passwordHash string
	var user api.User

	err := h.DB.QueryRowContext(r.Context(),
		`SELECT password_hash, id, email, first_name, last_name, role
		 FROM users WHERE email = $1 AND active = true`,
		req.Email,
	).Scan(&passwordHash, &user.Id, &user.Email, &user.FirstName, &user.LastName, &user.Role)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "ungültige Anmeldedaten")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "ungültige Anmeldedaten")
		return
	}

	accessToken, err := h.generateAccessToken(user.Id, string(user.Email), string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	refreshToken, err := generateRefreshToken(user.Id, h.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = h.DB.ExecContext(r.Context(),
		`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		user.Id, refreshTokenHash(refreshToken), expiresAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})

	writeJSON(w, http.StatusOK, api.LoginResponse{
		AccessToken: accessToken,
		User:        user,
	})
}

func (h *Server) PostApiV1AuthRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "kein Refresh Token")
		return
	}

	var userID int
	var expiresAt time.Time
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token = $1`,
		refreshTokenHash(cookie.Value),
	).Scan(&userID, &expiresAt)

	if err == sql.ErrNoRows || time.Now().After(expiresAt) {
		writeError(w, http.StatusUnauthorized, "ungültiger oder abgelaufener Refresh Token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	var email, role string
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT email, role FROM users WHERE id = $1 AND active = true`,
		userID,
	).Scan(&email, &role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Benutzer nicht gefunden oder inaktiv")
		return
	}

	// Rotate refresh tokens so a stolen cookie can only be used once.
	newRefreshToken, err := generateRefreshToken(userID, h.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}
	newExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM refresh_tokens WHERE token = $1`, refreshTokenHash(cookie.Value)); err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1,$2,$3)`, userID, refreshTokenHash(newRefreshToken), newExpiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: newRefreshToken, Path: "/api/v1/auth", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: newExpiresAt})

	accessToken, err := h.generateAccessToken(userID, email, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	writeJSON(w, http.StatusOK, api.RefreshResponse{AccessToken: accessToken})
}

func (h *Server) PostApiV1AuthLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		h.DB.ExecContext(r.Context(),
			`DELETE FROM refresh_tokens WHERE token = $1`, refreshTokenHash(cookie.Value))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *Server) GetApiV1AuthMe(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}

	var user api.User
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, role FROM users WHERE id = $1`,
		claims.UserID,
	).Scan(&user.Id, &user.Email, &user.FirstName, &user.LastName, &user.Role)

	if err != nil {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Server) PatchApiV1AuthMe(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht autorisiert")
		return
	}

	var req api.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültige Anfrage")
		return
	}

	_, err := h.DB.ExecContext(r.Context(),
		`UPDATE users SET
			first_name = COALESCE($1, first_name),
			last_name  = COALESCE($2, last_name),
			updated_at = NOW()
		WHERE id = $3`,
		req.FirstName, req.LastName, claims.UserID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	h.GetApiV1AuthMe(w, r)
}

func (h *Server) generateAccessToken(userID int, email, role string) (string, error) {
	claims := appmw.Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.JWTSecret)
}

func generateRefreshToken(userID int, jwtSecret []byte) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.Itoa(userID),
		ID:        uuid.NewString(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}
