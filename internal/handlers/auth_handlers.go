
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"medagent/internal/auth"
)

type AuthHandler struct {
	DB *pgxpool.Pool
}


type registerRequest struct {
	OrgID			string `json:"org_id"`
	Email			string `json:"email"`
	Password	string `json:"password"`
	Role			string `json:"role"`
}


func (h *AuthHandler) Register(w http.ResponseWriter, r*http.Request){

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req);
	err!=nil{
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	hash, err:=auth.HashPassword(req.Password)
	if err!=nil{
		http.Error(w, "failed to hash password",http.StatusInternalServerError)
		return
	}

	var userID string
	err = h.DB.QueryRow(r.Context(),`INSERT INTO users (org_id, email, password_hash, role) VALUES ($1, $2, $3, $4) RETURNING id`, req.OrgID, req.Email, hash, req.Role).Scan(&userID)
	if err!=nil{
		http.Error(w,"failed to create user",http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"user_id":userID})

}


type loginRequest struct {
	Email			string `json:"email"`
	Password	string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var userIDStr, orgIDStr, passwordHash, role string
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, org_id, password_hash, role FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userIDStr, &orgIDStr, &passwordHash, &role)
	if err != nil || !auth.CheckPassword(req.Password, passwordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(userID, orgID, role)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"role":  role,
	})
}






