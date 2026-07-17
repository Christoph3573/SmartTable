package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	appmw "schulapp/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB        *sql.DB
	JWTSecret []byte
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type loginResponse struct {
	AccessToken string       `json:"access_token"`
	User        userResponse `json:"user"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültige Anfrage")
		return
	}

	var user struct {
		ID           int
		Email        string
		PasswordHash string
		FirstName    string
		LastName     string
		Role         string
	}

	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, email, password_hash, first_name, last_name, role FROM users WHERE email = $1 AND active = true`,
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.Role)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "ungültige Anmeldedaten")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "ungültige Anmeldedaten")
		return
	}

	accessToken, err := h.generateAccessToken(user.ID, user.Email, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	refreshToken, err := h.generateRefreshToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = h.DB.ExecContext(r.Context(),
		`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		user.ID, refreshToken, expiresAt,
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
		Secure:   r.TLS != nil,
	})

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: accessToken,
		User: userResponse{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      user.Role,
		},
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "kein Refresh Token")
		return
	}

	var userID int
	var expiresAt time.Time
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token = $1`,
		cookie.Value,
	).Scan(&userID, &expiresAt)

	if err == sql.ErrNoRows || time.Now().After(expiresAt) {
		writeError(w, http.StatusUnauthorized, "abgelaufener Refresh Token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	var user struct {
		Email string
		Role  string
	}
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT email, role FROM users WHERE id = $1 AND active = true`,
		userID,
	).Scan(&user.Email, &user.Role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ungültige Anmeldedaten")
		return
	}

	accessToken, err := h.generateAccessToken(userID, user.Email, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "interner Fehler")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		h.DB.ExecContext(r.Context(),
			`DELETE FROM refresh_tokens WHERE token = $1`, cookie.Value)
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

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)

	var user userResponse
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, role FROM users WHERE id = $1`,
		claims.UserID,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Role)

	if err != nil {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	claims := appmw.GetClaims(r)

	var req struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
	}
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

	h.Me(w, r)
}

func (h *AuthHandler) generateAccessToken(userID int, email, role string) (string, error) {
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

func (h *AuthHandler) generateRefreshToken(userID int) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   string(rune(userID)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.JWTSecret)
}
