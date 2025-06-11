package database

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Signal represents a trading signal from TradingView or other sources
type Signal struct {
	ID          int             `json:"id" db:"id"`
	UUID        uuid.UUID       `json:"uuid" db:"uuid"`
	Source      string          `json:"source" db:"source"`
	Symbol      string          `json:"symbol" db:"symbol"`
	SignalType  string          `json:"signal_type" db:"signal_type"`
	Price       *float64        `json:"price,omitempty" db:"price"`
	StopLoss    *float64        `json:"stop_loss,omitempty" db:"stop_loss"`
	TakeProfit  *float64        `json:"take_profit,omitempty" db:"take_profit"`
	TP1         *float64        `json:"tp1,omitempty" db:"tp1"`
	TP2         *float64        `json:"tp2,omitempty" db:"tp2"`
	SL1         *float64        `json:"sl1,omitempty" db:"sl1"`
	SL2         *float64        `json:"sl2,omitempty" db:"sl2"`
	Payload     json.RawMessage `json:"payload" db:"payload"`
	Processed   bool            `json:"processed" db:"processed"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty" db:"processed_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// Trade represents an executed or pending trade
type Trade struct {
	ID             int              `json:"id" db:"id"`
	UUID           uuid.UUID        `json:"uuid" db:"uuid"`
	SignalID       *int             `json:"signal_id,omitempty" db:"signal_id"`
	ParentSignalID *int             `json:"parent_signal_id,omitempty" db:"parent_signal_id"`
	ParentTradeID  *int             `json:"parent_trade_id,omitempty" db:"parent_trade_id"`
	TradeType      string           `json:"trade_type" db:"trade_type"`
	Symbol         string           `json:"symbol" db:"symbol"`
	OrderType      string           `json:"order_type" db:"order_type"`
	Direction      string           `json:"direction" db:"direction"`
	Volume         float64          `json:"volume" db:"volume"`
	EntryPrice     *float64         `json:"entry_price,omitempty" db:"entry_price"`
	CurrentPrice   *float64         `json:"current_price,omitempty" db:"current_price"`
	StopLoss       *float64         `json:"stop_loss,omitempty" db:"stop_loss"`
	TakeProfit     *float64         `json:"take_profit,omitempty" db:"take_profit"`
	TP1            *float64         `json:"tp1,omitempty" db:"tp1"`
	TP2            *float64         `json:"tp2,omitempty" db:"tp2"`
	SL1            *float64         `json:"sl1,omitempty" db:"sl1"`
	SL2            *float64         `json:"sl2,omitempty" db:"sl2"`
	Status         string           `json:"status" db:"status"`
	MT5Ticket      *int64           `json:"mt5_ticket,omitempty" db:"mt5_ticket"`
	MT5Response    *json.RawMessage `json:"mt5_response,omitempty" db:"mt5_response"`
	ProfitLoss     float64          `json:"profit_loss" db:"profit_loss"`
	Commission     float64          `json:"commission" db:"commission"`
	Swap           float64          `json:"swap" db:"swap"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at" db:"updated_at"`
	ClosedAt       *time.Time       `json:"closed_at,omitempty" db:"closed_at"`
}

// SystemLog represents system events and logs
type SystemLog struct {
	ID        int             `json:"id" db:"id"`
	Level     string          `json:"level" db:"level"`
	Message   string          `json:"message" db:"message"`
	Component string          `json:"component" db:"component"`
	Context   json.RawMessage `json:"context,omitempty" db:"context"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// RiskEvent represents risk management events
type RiskEvent struct {
	ID          int             `json:"id" db:"id"`
	EventType   string          `json:"event_type" db:"event_type"`
	Description string          `json:"description" db:"description"`
	Severity    string          `json:"severity" db:"severity"`
	TradeID     *int            `json:"trade_id,omitempty" db:"trade_id"`
	SignalID    *int            `json:"signal_id,omitempty" db:"signal_id"`
	Context     json.RawMessage `json:"context,omitempty" db:"context"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// TradingViewWebhook represents the incoming webhook payload from TradingView
type TradingViewWebhook struct {
	Ticker     string          `json:"ticker"`
	Action     string          `json:"action"`
	Price      *float64        `json:"price,omitempty"`
	Entry      *float64        `json:"entry,omitempty"` // Alternative to price
	StopLoss   *float64        `json:"stop_loss,omitempty"`
	TakeProfit *float64        `json:"take_profit,omitempty"`
	TP1        *float64        `json:"tp1,omitempty"`
	TP2        *float64        `json:"tp2,omitempty"`
	Volume     *float64        `json:"volume,omitempty"`
	Message    string          `json:"message,omitempty"`
	Timestamp  json.RawMessage `json:"timestamp,omitempty"` // Flexible: accepts both number and string
}

// CreateSignalRequest represents the request to create a new signal
type CreateSignalRequest struct {
	Source     string          `json:"source"`
	Symbol     string          `json:"symbol"`
	SignalType string          `json:"signal_type"`
	Price      *float64        `json:"price,omitempty"`
	StopLoss   *float64        `json:"stop_loss,omitempty"`
	TakeProfit *float64        `json:"take_profit,omitempty"`
	TP1        *float64        `json:"tp1,omitempty"`
	TP2        *float64        `json:"tp2,omitempty"`
	SL1        *float64        `json:"sl1,omitempty"`
	SL2        *float64        `json:"sl2,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

// CreateTradeRequest represents the request to create a new trade
type CreateTradeRequest struct {
	SignalID       *int     `json:"signal_id,omitempty"`
	ParentSignalID *int     `json:"parent_signal_id,omitempty"`
	ParentTradeID  *int     `json:"parent_trade_id,omitempty"`
	TradeType      string   `json:"trade_type"`
	Symbol         string   `json:"symbol"`
	OrderType      string   `json:"order_type"`
	Direction      string   `json:"direction"`
	Volume         float64  `json:"volume"`
	EntryPrice     *float64 `json:"entry_price,omitempty"`
	StopLoss       *float64 `json:"stop_loss,omitempty"`
	TakeProfit     *float64 `json:"take_profit,omitempty"`
	TP1            *float64 `json:"tp1,omitempty"`
	TP2            *float64 `json:"tp2,omitempty"`
	SL1            *float64 `json:"sl1,omitempty"`
	SL2            *float64 `json:"sl2,omitempty"`
}

// UpdateTradeStatusRequest represents the request to update trade status
type UpdateTradeStatusRequest struct {
	Status       string           `json:"status"`
	MT5Ticket    *int64           `json:"mt5_ticket,omitempty"`
	MT5Response  *json.RawMessage `json:"mt5_response,omitempty"`
	EntryPrice   *float64         `json:"entry_price,omitempty"`
	CurrentPrice *float64         `json:"current_price,omitempty"`
	ProfitLoss   *float64         `json:"profit_loss,omitempty"`
	Commission   *float64         `json:"commission,omitempty"`
	Swap         *float64         `json:"swap,omitempty"`
}
