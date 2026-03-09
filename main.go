package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/handlers"
	"jarvis-gateway/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", handlers.Health)

	// Documentation
	mux.HandleFunc("GET /docs", handlers.Docs(cfg))
	mux.HandleFunc("GET /docs/", handlers.Docs(cfg))

	// Webhook endpoints
	mux.HandleFunc("POST /api/calendar", middleware.Auth(cfg, handlers.Calendar(cfg)))
	mux.HandleFunc("POST /api/gmail", middleware.Auth(cfg, handlers.Gmail(cfg)))
	mux.HandleFunc("POST /api/github", middleware.Auth(cfg, handlers.GitHub(cfg)))
	mux.HandleFunc("POST /api/custom", middleware.Auth(cfg, handlers.Custom(cfg)))

	// Logging middleware
	handler := middleware.Logger(mux)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		server.Close()
	}()

	log.Printf("JARVIS Gateway starting on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
