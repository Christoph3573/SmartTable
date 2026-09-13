package handler

import (
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	appmw "schulapp/internal/middleware"
)

// HandleWS upgrades to a WebSocket used to push chat messages and unread
// updates. Browsers can't set custom headers on the WebSocket handshake, so
// the JWT travels as a query param instead of the usual Authorization header.
func (h *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	claims := &appmw.Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.JWTSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	h.Hub.Serve(w, r, claims.UserID)
}

// channelMemberIDs returns every member of a chat channel, used to know who
// to push a new message / unread update to.
func (h *Server) channelMemberIDs(channelID int) []int {
	rows, err := h.DB.Query(`SELECT user_id FROM chat_members WHERE channel_id=$1`, channelID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
