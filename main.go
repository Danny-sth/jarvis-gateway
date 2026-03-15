package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/credentials"
	"jarvis-gateway/internal/db"
	"jarvis-gateway/internal/handlers"
	"jarvis-gateway/internal/middleware"
	"jarvis-gateway/internal/rbac"
	"jarvis-gateway/internal/session"
	"jarvis-gateway/internal/vtoroy"
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

	// Background goroutine for session cleanup (every hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			count, err := dbClient.DeleteExpiredSessions()
			if err != nil {
				log.Printf("[cleanup] Error deleting expired sessions: %v", err)
			} else if count > 0 {
				log.Printf("[cleanup] Deleted %d expired sessions", count)
			}
		}
	}()

	// Initialize services
	rbacService := rbac.NewService(dbClient.DB())
	sessionService := session.NewService(dbClient.DB())
	sessionAdapter := session.NewHandlerAdapter(sessionService)
	credService := credentials.NewService(dbClient.DB())
	vtoroyClient := vtoroy.NewClient(cfg)

	// Create Telegram handler with full dependencies
	telegramDeps := &handlers.TelegramDeps{
		Config:         cfg,
		VtoroyClient:   vtoroyClient,
		RBACService:    rbacService,
		SessionService: sessionAdapter,
		CredService:    credService,
	}

	// Google OAuth dependencies
	oauthDeps := &handlers.GoogleOAuthDeps{
		Config:      cfg,
		CredService: credService,
		SendMessage: func(chatID int64, text string) error {
			return handlers.SendTelegramMessage(cfg, chatID, text)
		},
	}

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
	mux.HandleFunc("POST /api/telegram/webhook", handlers.TelegramWithDeps(telegramDeps))

	// Telegram send endpoint (for vtoroy scheduler, internal use)
	mux.HandleFunc("POST /api/telegram/send", middleware.Auth(cfg, handlers.TelegramSend(cfg)))

	// QR Auth endpoints for mobile app
	mux.HandleFunc("POST /api/auth/qr/generate", middleware.Auth(cfg, handlers.QRGenerate(cfg, dbClient)))
	mux.HandleFunc("POST /api/auth/qr/verify", handlers.QRVerify(cfg, dbClient)) // Public endpoint

	// Voice endpoint for mobile app (protected with mobile token)
	mux.HandleFunc("POST /api/voice", middleware.MobileAuth(dbClient, handlers.Voice(cfg, dbClient)))

	// Google OAuth endpoints
	mux.HandleFunc("GET /api/auth/google/callback", handlers.GoogleOAuthCallback(oauthDeps))
	mux.HandleFunc("GET /api/auth/google/link", middleware.Auth(cfg, handlers.GetOAuthLinkHandler(cfg)))
	mux.HandleFunc("POST /api/oauth/google/initiate", middleware.Auth(cfg, handlers.InitiateOAuthHandler(oauthDeps)))

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
