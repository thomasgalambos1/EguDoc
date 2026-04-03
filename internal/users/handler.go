// internal/users/handler.go
package users

import (
	"encoding/json"
	"net/http"

	"github.com/eguilde/egudoc/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/me", h.GetMe)
	return r
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}

	user, err := h.svc.GetOrCreate(r.Context(), claims, ip)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		// response already started, can't change status
		return
	}
}
