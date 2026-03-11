package openclaw

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"jarvis-gateway/internal/config"
)

// WSClient communicates with OpenClaw Gateway via WebSocket RPC
type WSClient struct {
	url         string
	token       string
	agentID     string
	defaultUser string

	conn     *websocket.Conn
	connMu   sync.Mutex
	reqID    atomic.Uint64
	pending  map[string]chan *RPCResponse
	pendingMu sync.Mutex

	reconnectInterval time.Duration
	maxReconnectWait  time.Duration
}

// RPC Message Types
type RPCRequest struct {
	Type   string      `json:"type"`   // "req"
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type RPCResponse struct {
	Type    string          `json:"type"` // "res" or "event"
	ID      string          `json:"id,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	Event   string          `json:"event,omitempty"`
}

type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChatSendParams for chat.send RPC method
type ChatSendParams struct {
	IdempotencyKey string `json:"idempotencyKey"`
	SessionKey     string `json:"sessionKey,omitempty"`
	Message        string `json:"message"`
	Channel        string `json:"channel,omitempty"`
	Target         string `json:"target,omitempty"`
	Deliver        bool   `json:"deliver,omitempty"`
}

// AgentResponse from chat.send
type AgentResponse struct {
	Action  string `json:"action"`
	Payload struct {
		Text      string `json:"text"`
		MessageID string `json:"messageId,omitempty"`
	} `json:"payload"`
}

func NewWSClient(cfg *config.Config) *WSClient {
	wsURL := cfg.OpenClaw.GatewayURL
	// Convert http:// to ws://
	if len(wsURL) > 7 && wsURL[:7] == "http://" {
		wsURL = "ws://" + wsURL[7:]
	} else if len(wsURL) > 8 && wsURL[:8] == "https://" {
		wsURL = "wss://" + wsURL[8:]
	}

	return &WSClient{
		url:               wsURL,
		token:             cfg.OpenClaw.Token,
		agentID:           cfg.OpenClaw.AgentID,
		defaultUser:       "telegram:" + cfg.TelegramChatID,
		pending:           make(map[string]chan *RPCResponse),
		reconnectInterval: 1 * time.Second,
		maxReconnectWait:  30 * time.Second,
	}
}

// ConnectParams for connect RPC handshake
type ConnectParams struct {
	MinProtocol int                    `json:"minProtocol"`
	MaxProtocol int                    `json:"maxProtocol"`
	Client      ClientInfo             `json:"client"`
	Role        string                 `json:"role"`
	Scopes      []string               `json:"scopes"`
	Caps        []string               `json:"caps"`
	Commands    []string               `json:"commands"`
	Permissions map[string]interface{} `json:"permissions"`
	Auth        AuthParams             `json:"auth"`
	Locale      string                 `json:"locale"`
	UserAgent   string                 `json:"userAgent"`
}

type ClientInfo struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Mode     string `json:"mode"`
}

type AuthParams struct {
	Token string `json:"token"`
}

// Connect establishes WebSocket connection to OpenClaw Gateway
func (c *WSClient) Connect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return nil // Already connected
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn

	// Perform connect handshake
	if err := c.doHandshake(); err != nil {
		c.conn.Close()
		c.conn = nil
		return fmt.Errorf("handshake: %w", err)
	}

	go c.readLoop()

	log.Printf("[openclaw] Connected to WebSocket: %s", c.url)
	return nil
}

// doHandshake performs the connect RPC handshake
func (c *WSClient) doHandshake() error {
	// Send connect request
	reqID := fmt.Sprintf("%d", c.reqID.Add(1))
	connectReq := RPCRequest{
		Type:   "req",
		ID:     reqID,
		Method: "connect",
		Params: ConnectParams{
			MinProtocol: 3,
			MaxProtocol: 3,
			Client: ClientInfo{
				ID:       "cli",
				Version:  "2026.3.2",
				Platform: "linux",
				Mode:     "backend",
			},
			Role:        "operator",
			Scopes:      []string{"operator.read", "operator.write", "operator.admin"},
			Caps:        []string{},
			Commands:    []string{},
			Permissions: map[string]interface{}{},
			Auth: AuthParams{
				Token: c.token,
			},
			Locale:    "en-US",
			UserAgent: "jarvis-gateway/1.0.0",
		},
	}

	data, err := json.Marshal(connectReq)
	if err != nil {
		return fmt.Errorf("marshal connect: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write connect: %w", err)
	}

	// Wait for connect response (with timeout)
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer c.conn.SetReadDeadline(time.Time{})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read connect response: %w", err)
		}

		var resp RPCResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			log.Printf("[openclaw] Handshake: failed to parse: %v", err)
			continue
		}

		// Skip events, wait for response
		if resp.Type == "event" {
			log.Printf("[openclaw] Handshake event: %s", resp.Event)
			continue
		}

		if resp.Type == "res" && resp.ID == reqID {
			if !resp.OK {
				errMsg := "unknown error"
				if resp.Error != nil {
					errMsg = resp.Error.Message
				}
				return fmt.Errorf("connect rejected: %s", errMsg)
			}
			log.Printf("[openclaw] Handshake successful")
			return nil
		}
	}
}

// Close closes the WebSocket connection
func (c *WSClient) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// readLoop reads messages from WebSocket and dispatches to pending requests
func (c *WSClient) readLoop() {
	for {
		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[openclaw] WebSocket read error: %v", err)
			c.handleDisconnect()
			return
		}

		var resp RPCResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			log.Printf("[openclaw] Failed to parse response: %v", err)
			continue
		}

		// Handle response
		if resp.Type == "res" && resp.ID != "" {
			c.pendingMu.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				ch <- &resp
				delete(c.pending, resp.ID)
			}
			c.pendingMu.Unlock()
		}

		// Handle events (optional logging)
		if resp.Type == "event" {
			log.Printf("[openclaw] Event: %s", resp.Event)
		}
	}
}

// handleDisconnect handles WebSocket disconnection
func (c *WSClient) handleDisconnect() {
	c.connMu.Lock()
	c.conn = nil
	c.connMu.Unlock()

	// Clear pending requests
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	log.Printf("[openclaw] Disconnected, will reconnect on next request")
}

// ensureConnected ensures WebSocket is connected
func (c *WSClient) ensureConnected() error {
	c.connMu.Lock()
	connected := c.conn != nil
	c.connMu.Unlock()

	if connected {
		return nil
	}

	return c.Connect()
}

// call sends RPC request and waits for response
func (c *WSClient) call(method string, params interface{}, timeout time.Duration) (*RPCResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	reqID := fmt.Sprintf("%d", c.reqID.Add(1))
	req := RPCRequest{
		Type:   "req",
		ID:     reqID,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create response channel
	respCh := make(chan *RPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[reqID] = respCh
	c.pendingMu.Unlock()

	// Send request
	c.connMu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.connMu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write message: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		if resp == nil {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %s: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-time.After(timeout):
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("request timeout")
	}
}

// SendMessage sends a message to the agent and delivers to default user
func (c *WSClient) SendMessage(message string) error {
	_, err := c.Send(message, c.defaultUser)
	return err
}

// Send sends a message to the agent for a specific user
func (c *WSClient) Send(message, userID string) (string, error) {
	// Generate unique idempotency key
	idempotencyKey := fmt.Sprintf("%d-%d", time.Now().UnixNano(), c.reqID.Load())

	// Parse userID format: "telegram:764733417" -> sessionKey "agent:main:telegram:direct:764733417"
	sessionKey := fmt.Sprintf("agent:%s:%s", c.agentID, userID)
	// Insert "direct:" before the peerId if format is channel:peerId
	if parts := splitOnce(userID, ":"); len(parts) == 2 {
		sessionKey = fmt.Sprintf("agent:%s:%s:direct:%s", c.agentID, parts[0], parts[1])
	}

	params := ChatSendParams{
		IdempotencyKey: idempotencyKey,
		SessionKey:     sessionKey,
		Message:        message,
		Deliver:        true,
	}

	resp, err := c.call("chat.send", params, 120*time.Second)
	if err != nil {
		return "", err
	}

	// Parse response payload
	var agentResp AgentResponse
	if err := json.Unmarshal(resp.Payload, &agentResp); err != nil {
		// Return raw payload as string if parsing fails
		return string(resp.Payload), nil
	}

	return agentResp.Payload.Text, nil
}

// SendDirect sends a message directly to a channel (bypassing agent)
func (c *WSClient) SendDirect(message, channel, target string) error {
	params := map[string]interface{}{
		"channel": channel,
		"target":  target,
		"message": message,
	}

	_, err := c.call("message.send", params, 30*time.Second)
	return err
}

// splitOnce splits string on first occurrence of sep
func splitOnce(s, sep string) []string {
	idx := -1
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			idx = i
			break
		}
	}
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+len(sep):]}
}
