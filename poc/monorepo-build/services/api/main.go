package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"monorepo-build/shared/models"
)

const (
	DefaultPort    = "8080"
	ReadTimeout    = 15 * time.Second
	WriteTimeout   = 15 * time.Second
	IdleTimeout    = 60 * time.Second
	ShutdownPeriod = 30 * time.Second
)

// Server represents the API server
type Server struct {
	srv    *http.Server
	router *http.ServeMux
	users  map[string]*models.User
}

// NewServer creates a new API server
func NewServer(port string) *Server {
	mux := http.NewServeMux()
	s := &Server{
		router: mux,
		users:  make(map[string]*models.User),
		srv: &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  ReadTimeout,
			WriteTimeout: WriteTimeout,
			IdleTimeout:  IdleTimeout,
		},
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up all HTTP routes
func (s *Server) registerRoutes() {
	s.router.HandleFunc("/health", s.handleHealth)
	s.router.HandleFunc("/users", s.handleUsers)
	s.router.HandleFunc("/users/create", s.handleCreateUser)
	s.router.HandleFunc("/users/list", s.handleListUsers)
	s.router.HandleFunc("/stats", s.handleStats)
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	log.Printf("API Server starting on %s", s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("API Server shutting down...")
	return s.srv.Shutdown(ctx)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	server := NewServer(port)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownPeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("API Server exited")
}
