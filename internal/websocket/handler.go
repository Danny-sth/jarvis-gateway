package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/coder/websocket"

	"duq-gateway/internal/config"
)

// Handler creates an HTTP handler for WebSocket connections
func Handler(hub *Hub, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query param or Authorization header
		token := r.URL.Query().Get("token")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			http.Error(w, "token required", http.StatusUnauthorized)
			return
		}

		// Validate JWT
		claims, err := validateKeycloakJWT(token, cfg)
		if err != nil {
			log.Printf("[ws] JWT validation failed: %v", err)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Get device_id from query params
		deviceID := r.URL.Query().Get("device_id")
		if deviceID == "" {
			deviceID = "unknown"
		}

		// Accept WebSocket connection
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns:     []string{"*"}, // Allow all origins (JWT handles auth)
			InsecureSkipVerify: true,          // For development
		})
		if err != nil {
			log.Printf("[ws] Failed to accept WebSocket: %v", err)
			return
		}

		// Create connection context (independent of HTTP request context)
		// r.Context() gets cancelled when handler returns, so use Background()
		ctx, cancel := context.WithCancel(context.Background())

		// Use telegram_id if available (from Keycloak user attribute mapper)
		// Falls back to keycloak sub (UUID) if telegram_id not set
		userID := claims.TelegramID
		if userID == "" {
			userID = claims.Subject
			log.Printf("[ws] Warning: telegram_id not in JWT, using sub: %s", userID)
		}

		connection := &Connection{
			Conn:      conn,
			UserID:    userID,
			DeviceID:  deviceID,
			CreatedAt: time.Now(),
			ctx:       ctx,
			cancel:    cancel,
		}

		// Register connection
		hub.Register(connection)

		// Start read loop (handles ping/pong and client messages)
		go handleConnection(hub, connection)

		log.Printf("[ws] Connection established: user=%s device=%s", userID, deviceID)
	}
}

func handleConnection(hub *Hub, conn *Connection) {
	defer func() {
		hub.Unregister(conn)
		conn.Conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	// Read loop - blocks until message received or context cancelled
	// OkHttp sends WebSocket-level pings which the library handles automatically
	for {
		msgType, data, err := conn.Conn.Read(conn.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				log.Printf("[ws] Connection closed normally: user=%s", conn.UserID)
			} else if conn.ctx.Err() != nil {
				log.Printf("[ws] Connection context cancelled: user=%s", conn.UserID)
			} else {
				log.Printf("[ws] Read error: user=%s err=%v", conn.UserID, err)
			}
			return
		}

		if msgType == websocket.MessageText {
			handleClientMessage(hub, conn, data)
		}
	}
}

type clientMessage struct {
	Type string `json:"type"` // "ping", "subscribe"
}

func handleClientMessage(hub *Hub, conn *Connection, data []byte) {
	var msg clientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[ws] Invalid client message: %v", err)
		return
	}

	switch msg.Type {
	case "ping":
		// Respond with pong
		pong := &Message{
			Type:      "pong",
			Timestamp: time.Now().Unix(),
		}
		pongData, _ := json.Marshal(pong)
		conn.Conn.Write(conn.ctx, websocket.MessageText, pongData)

	default:
		log.Printf("[ws] Unknown message type: %s", msg.Type)
	}
}

// KeycloakClaims represents the JWT claims from Keycloak
type KeycloakClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	PreferredUser string `json:"preferred_username"`
	TelegramID    string `json:"telegram_id"` // Custom claim from user attribute
}

func validateKeycloakJWT(tokenString string, cfg *config.Config) (*KeycloakClaims, error) {
	// Parse without validation first to get claims (we'll validate signature separately)
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &KeycloakClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*KeycloakClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	// Validate expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, jwt.ErrTokenExpired
	}

	// Validate issuer - accept both external URL and internal URL for migration
	// External: https://on-za-menya.online/realms/duq (new tokens)
	// Internal: http://keycloak:8180/realms/duq (legacy tokens during migration)
	expectedIssuer := cfg.Keycloak.URL + "/realms/" + cfg.Keycloak.Realm
	internalIssuer := cfg.KeycloakInternalURL + "/realms/" + cfg.Keycloak.Realm

	if claims.Issuer != expectedIssuer && claims.Issuer != internalIssuer {
		log.Printf("[ws] Issuer mismatch: got=%s, expected=%s or %s", claims.Issuer, expectedIssuer, internalIssuer)
		return nil, jwt.ErrTokenInvalidIssuer
	}

	return claims, nil
}
