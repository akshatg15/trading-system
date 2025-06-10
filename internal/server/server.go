package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"trading-system/internal/config"
	"trading-system/internal/database"
	"trading-system/internal/signals"
)

// Server handles HTTP requests
type Server struct {
	config          *config.Config
	db              *database.DB
	signalProcessor *signals.Processor
}

// New creates a new server
func New(cfg *config.Config, db *database.DB, processor *signals.Processor) *Server {
	return &Server{
		config:          cfg,
		db:              db,
		signalProcessor: processor,
	}
}

// Router returns HTTP router
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/webhook/tradingview", s.handleTradingViewWebhook)
	mux.HandleFunc("/trades", s.handleGetTrades)
	mux.HandleFunc("/positions", s.handleGetPositions)
	mux.HandleFunc("/mt5/status", s.handleMT5Status)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) handleTradingViewWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify webhook signature if secret is configured
	// if s.config.Server.WebhookSecret != "" {
	// 	if !s.verifyWebhookSignature(r, body) {
	// 		log.Printf("Invalid webhook signature")
	// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	// 		return
	// 	}
	// }

	// Process the webhook
	signal, err := s.signalProcessor.ProcessWebhook(r.Context(), body, "tradingview")
	if err != nil {
		log.Printf("Error processing webhook: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully processed webhook, created signal %d", signal.ID)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"signal_id": signal.ID,
		"message":   "Signal processed successfully",
	})
}

// verifyWebhookSignature verifies the webhook signature for security
func (s *Server) verifyWebhookSignature(r *http.Request, body []byte) bool {
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Hub-Signature-256")
	}

	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(s.config.Server.WebhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func (s *Server) handleGetTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trades, err := s.db.GetOpenTrades(r.Context())
	if err != nil {
		log.Printf("Error getting trades: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trades)
}

func (s *Server) handleGetPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get positions from MT5 via signal processor
	mt5Client := s.signalProcessor.GetMT5Client()
	positions, err := mt5Client.GetPositions(r.Context())
	if err != nil {
		log.Printf("Error getting MT5 positions: %v", err)
		http.Error(w, "Failed to get positions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(positions)
}

func (s *Server) handleMT5Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mt5Client := s.signalProcessor.GetMT5Client()

	status := map[string]interface{}{
		"connected": mt5Client.IsConnected(r.Context()),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Get account info if connected
	if status["connected"].(bool) {
		accountInfo, err := mt5Client.GetAccountInfo(r.Context())
		if err == nil {
			status["account"] = accountInfo
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
