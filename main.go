package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/db"
	"jarvis-gateway/internal/handlers"
	"jarvis-gateway/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection
	dbClient, err := db.New(db.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Name:     cfg.Database.Name,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", handlers.Health)

	// Documentation (protected with BasicAuth)
	mux.HandleFunc("GET /docs", middleware.BasicAuth(cfg, handlers.Docs(cfg)))
	mux.HandleFunc("GET /docs/", middleware.BasicAuth(cfg, handlers.Docs(cfg)))

	// Webhook endpoints
	mux.HandleFunc("POST /api/calendar", middleware.Auth(cfg, handlers.Calendar(cfg)))
	mux.HandleFunc("POST /api/gmail", middleware.Auth(cfg, handlers.Gmail(cfg)))
	mux.HandleFunc("POST /api/github", handlers.GitHub(cfg)) // Uses HMAC signature verification
	mux.HandleFunc("POST /api/custom", middleware.Auth(cfg, handlers.Custom(cfg)))

	// Telegram webhook (no auth - Telegram sends updates directly)
	mux.HandleFunc("POST /api/telegram/webhook", handlers.Telegram(cfg))

	// QR Auth endpoints for mobile app
	mux.HandleFunc("POST /api/auth/qr/generate", middleware.Auth(cfg, handlers.QRGenerate(cfg, dbClient)))
	mux.HandleFunc("POST /api/auth/qr/verify", handlers.QRVerify(cfg, dbClient)) // Public endpoint

	// Voice endpoint for mobile app (protected with mobile token)
	mux.HandleFunc("POST /api/voice", middleware.MobileAuth(dbClient, handlers.Voice(cfg, dbClient)))

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
