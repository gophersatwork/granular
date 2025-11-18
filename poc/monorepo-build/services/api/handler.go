package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"monorepo-build/shared/models"
	"monorepo-build/shared/utils"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains response metadata
type Meta struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
	Version   string    `json:"version"`
}

var (
	requestCounter int
	counterMutex   sync.Mutex
)

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Success: true,
		Data: map[string]interface{}{
			"status":  "healthy",
			"uptime":  time.Now().Format(time.RFC3339),
			"version": "1.0.0",
		},
		Meta: buildMeta(),
	}
	writeJSON(w, http.StatusOK, response)
}

// handleUsers handles generic user operations
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListUsers(w, r)
	case http.MethodPost:
		s.handleCreateUser(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleCreateUser creates a new user
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user := &models.User{
		ID:        utils.GenerateID("user"),
		Email:     utils.SanitizeEmail(req.Email),
		Name:      req.Name,
		Role:      req.Role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	if err := user.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.users[user.ID] = user

	response := Response{
		Success: true,
		Data:    user,
		Meta:    buildMeta(),
	}
	writeJSON(w, http.StatusCreated, response)
}

// handleListUsers returns all users
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users := make([]*models.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}

	response := Response{
		Success: true,
		Data: map[string]interface{}{
			"users": users,
			"count": len(users),
		},
		Meta: buildMeta(),
	}
	writeJSON(w, http.StatusOK, response)
}

// handleStats returns server statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	counterMutex.Lock()
	count := requestCounter
	counterMutex.Unlock()

	stats := map[string]interface{}{
		"total_users":    len(s.users),
		"total_requests": count,
		"timestamp":      time.Now().Format(time.RFC3339),
		"uptime_seconds": time.Since(time.Now().Add(-1 * time.Hour)).Seconds(),
	}

	response := Response{
		Success: true,
		Data:    stats,
		Meta:    buildMeta(),
	}
	writeJSON(w, http.StatusOK, response)
}

// Helper functions

func buildMeta() *Meta {
	counterMutex.Lock()
	requestCounter++
	reqID := fmt.Sprintf("req_%d", requestCounter)
	counterMutex.Unlock()

	return &Meta{
		Timestamp: time.Now(),
		RequestID: reqID,
		Version:   "1.0.0",
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	response := Response{
		Success: false,
		Error:   message,
		Meta:    buildMeta(),
	}
	writeJSON(w, status, response)
}

// Demo modification at Thu Nov 13 11:45:21 PM -03 2025
