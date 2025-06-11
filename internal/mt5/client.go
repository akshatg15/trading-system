package mt5

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"trading-system/internal/config"
)

// Client handles communication with MT5 via HTTP bridge
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	retries    int
	retryDelay time.Duration
}

// NewClient creates a new MT5 client
func NewClient(cfg *config.MT5Config) *Client {
	return &Client{
		baseURL: cfg.Endpoint,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
		timeout:    time.Duration(cfg.TimeoutSeconds) * time.Second,
		retries:    cfg.RetryAttempts,
		retryDelay: time.Duration(cfg.RetryDelayMs) * time.Millisecond,
	}
}

// TradeRequest represents a trade execution request
type TradeRequest struct {
	Symbol     string  `json:"symbol"`
	Action     string  `json:"action"`     // "buy", "sell", "close"
	Volume     float64 `json:"volume"`     // lot size
	Price      float64 `json:"price,omitempty"`      // for limit orders
	StopLoss   float64 `json:"stop_loss,omitempty"`
	TakeProfit float64 `json:"take_profit,omitempty"`
	TP1        float64 `json:"tp1,omitempty"`         // First take profit level
	TP2        float64 `json:"tp2,omitempty"`         // Second take profit level
	OrderType  string  `json:"order_type"` // "market", "limit", "stop"
	Comment    string  `json:"comment,omitempty"`
	Magic      int     `json:"magic,omitempty"` // EA magic number
}

// TradeResponse represents MT5 trade execution response
type TradeResponse struct {
	Success              bool    `json:"success"`
	Ticket               int64   `json:"ticket,omitempty"`               // Single ticket (legacy)
	Tickets              []int64 `json:"tickets,omitempty"`              // Multiple tickets for partial TP
	Price                float64 `json:"price,omitempty"`
	Volume               float64 `json:"volume,omitempty"`
	Volumes              []float64 `json:"volumes,omitempty"`             // Multiple volumes for partial TP
	Prices               []float64 `json:"prices,omitempty"`              // Multiple prices for partial TP
	ErrorCode            int     `json:"error_code,omitempty"`
	ErrorMsg             string  `json:"error_msg,omitempty"`
	Commission           float64 `json:"commission,omitempty"`
	Swap                 float64 `json:"swap,omitempty"`
	Profit               float64 `json:"profit,omitempty"`
	PartialTPStrategy    bool    `json:"partial_tp_strategy,omitempty"`  // Indicates if partial TP was used
	TP1Ticket            int64   `json:"tp1_ticket,omitempty"`           // TP1 position ticket
	TP2Ticket            int64   `json:"tp2_ticket,omitempty"`           // TP2 position ticket
}

// OrderInfo represents pending order information
type OrderInfo struct {
	Ticket     int64   `json:"ticket"`
	Symbol     string  `json:"symbol"`
	Volume     float64 `json:"volume"`
	Type       string  `json:"type"`        // "buy_limit", "sell_limit", etc.
	Price      float64 `json:"price"`
	StopLoss   float64 `json:"stop_loss"`
	TakeProfit float64 `json:"take_profit"`
	Comment    string  `json:"comment"`
	OpenTime   string  `json:"open_time"`
}

// PositionInfo represents current position information
type PositionInfo struct {
	Ticket     int64   `json:"ticket"`
	Symbol     string  `json:"symbol"`
	Volume     float64 `json:"volume"`
	Type       string  `json:"type"`        // "buy", "sell"
	OpenPrice  float64 `json:"open_price"`
	CurrentPrice float64 `json:"current_price"`
	StopLoss   float64 `json:"stop_loss"`
	TakeProfit float64 `json:"take_profit"`
	Profit     float64 `json:"profit"`
	Commission float64 `json:"commission"`
	Swap       float64 `json:"swap"`
	Comment    string  `json:"comment"`
	OpenTime   string  `json:"open_time"`
}

// AccountInfo represents MT5 account information
type AccountInfo struct {
	Balance    float64 `json:"balance"`
	Equity     float64 `json:"equity"`
	Margin     float64 `json:"margin"`
	FreeMargin float64 `json:"free_margin"`
	Currency   string  `json:"currency"`
	Leverage   int     `json:"leverage"`
	Connected  bool    `json:"connected"`
}

// SendTrade sends a trade request to MT5
func (c *Client) SendTrade(ctx context.Context, req *TradeRequest) (*TradeResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= c.retries; attempt++ {
		resp, err := c.sendTradeRequest(ctx, req)
		if err == nil {
			return resp, nil
		}
		
		lastErr = err
		if attempt < c.retries {
			time.Sleep(c.retryDelay)
		}
	}
	
	return nil, fmt.Errorf("failed to send trade after %d attempts: %w", c.retries+1, lastErr)
}

// sendTradeRequest performs a single trade request
func (c *Client) sendTradeRequest(ctx context.Context, req *TradeRequest) (*TradeResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trade request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/trade", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MT5 bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	var tradeResp TradeResponse
	if err := json.Unmarshal(body, &tradeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trade response: %w", err)
	}

	return &tradeResp, nil
}

// GetPositions retrieves all open positions
func (c *Client) GetPositions(ctx context.Context) ([]*PositionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/positions", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MT5 bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	var positions []*PositionInfo
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal positions: %w", err)
	}

	return positions, nil
}

// GetPositionCount retrieves the number of open positions efficiently
func (c *Client) GetPositionCount(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/position-count", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("MT5 bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	var countResp struct {
		Count     int    `json:"count"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &countResp); err != nil {
		return 0, fmt.Errorf("failed to unmarshal position count: %w", err)
	}

	return countResp.Count, nil
}

// GetAccountInfo retrieves account information
func (c *Client) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/account", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MT5 bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	var account AccountInfo
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account info: %w", err)
	}

	return &account, nil
}

// ClosePosition closes a specific position by ticket
func (c *Client) ClosePosition(ctx context.Context, ticket int64) (*TradeResponse, error) {
	req := &TradeRequest{
		Action: "close",
		Magic:  int(ticket),
	}
	
	return c.SendTrade(ctx, req)
}

// IsConnected checks if MT5 bridge is available
func (c *Client) IsConnected(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetOrders retrieves all pending orders
func (c *Client) GetOrders(ctx context.Context) ([]*OrderInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/orders", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MT5 bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	var orders []*OrderInfo
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal orders: %w", err)
	}

	return orders, nil
}

// GetOrderCount retrieves the number of pending orders efficiently
func (c *Client) GetOrderCount(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/order-count", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("MT5 bridge returned status %d: %s", resp.StatusCode, string(body))
	}

	var countResp struct {
		Count     int    `json:"count"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &countResp); err != nil {
		return 0, fmt.Errorf("failed to unmarshal order count: %w", err)
	}

	return countResp.Count, nil
} 