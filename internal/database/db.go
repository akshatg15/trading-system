package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"trading-system/internal/config"

	_ "github.com/lib/pq"
)

// DB wraps the database connection and provides repository methods
type DB struct {
	conn *sql.DB
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig) (*DB, error) {
	conn, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(cfg.MaxConnections)
	conn.SetMaxIdleConns(cfg.MaxConnections / 2)
	conn.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateSignal inserts a new signal into the database
func (db *DB) CreateSignal(ctx context.Context, req *CreateSignalRequest) (*Signal, error) {
	query := `
		INSERT INTO signals (source, symbol, signal_type, price, stop_loss, take_profit, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, uuid, source, symbol, signal_type, price, stop_loss, take_profit, 
		          payload, processed, processed_at, created_at, updated_at
	`

	signal := &Signal{}
	err := db.conn.QueryRowContext(
		ctx, query,
		req.Source, req.Symbol, req.SignalType, req.Price, req.StopLoss, req.TakeProfit, req.Payload,
	).Scan(
		&signal.ID, &signal.UUID, &signal.Source, &signal.Symbol, &signal.SignalType,
		&signal.Price, &signal.StopLoss, &signal.TakeProfit, &signal.Payload,
		&signal.Processed, &signal.ProcessedAt, &signal.CreatedAt, &signal.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create signal: %w", err)
	}

	return signal, nil
}

// GetUnprocessedSignals retrieves all unprocessed signals
func (db *DB) GetUnprocessedSignals(ctx context.Context) ([]*Signal, error) {
	query := `
		SELECT id, uuid, source, symbol, signal_type, price, stop_loss, take_profit,
		       payload, processed, processed_at, created_at, updated_at
		FROM signals 
		WHERE processed = false 
		ORDER BY created_at ASC
	`

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed signals: %w", err)
	}
	defer rows.Close()

	var signals []*Signal
	for rows.Next() {
		signal := &Signal{}
		err := rows.Scan(
			&signal.ID, &signal.UUID, &signal.Source, &signal.Symbol, &signal.SignalType,
			&signal.Price, &signal.StopLoss, &signal.TakeProfit, &signal.Payload,
			&signal.Processed, &signal.ProcessedAt, &signal.CreatedAt, &signal.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan signal: %w", err)
		}
		signals = append(signals, signal)
	}

	return signals, nil
}

// MarkSignalProcessed marks a signal as processed
func (db *DB) MarkSignalProcessed(ctx context.Context, signalID int) error {
	query := `
		UPDATE signals 
		SET processed = true, processed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := db.conn.ExecContext(ctx, query, signalID)
	if err != nil {
		return fmt.Errorf("failed to mark signal as processed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("signal with ID %d not found", signalID)
	}

	return nil
}

// CreateTrade inserts a new trade into the database
func (db *DB) CreateTrade(ctx context.Context, req *CreateTradeRequest) (*Trade, error) {
	query := `
		INSERT INTO trades (signal_id, symbol, order_type, direction, volume, entry_price, stop_loss, take_profit)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, uuid, signal_id, symbol, order_type, direction, volume, entry_price,
		          current_price, stop_loss, take_profit, status, mt5_ticket, mt5_response,
		          profit_loss, commission, swap, created_at, updated_at, closed_at
	`

	trade := &Trade{}
	err := db.conn.QueryRowContext(
		ctx, query,
		req.SignalID, req.Symbol, req.OrderType, req.Direction, req.Volume,
		req.EntryPrice, req.StopLoss, req.TakeProfit,
	).Scan(
		&trade.ID, &trade.UUID, &trade.SignalID, &trade.Symbol, &trade.OrderType,
		&trade.Direction, &trade.Volume, &trade.EntryPrice, &trade.CurrentPrice,
		&trade.StopLoss, &trade.TakeProfit, &trade.Status, &trade.MT5Ticket,
		&trade.MT5Response, &trade.ProfitLoss, &trade.Commission, &trade.Swap,
		&trade.CreatedAt, &trade.UpdatedAt, &trade.ClosedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create trade: %w", err)
	}

	return trade, nil
}

// UpdateTradeStatus updates the status and details of a trade
func (db *DB) UpdateTradeStatus(ctx context.Context, tradeID int, req *UpdateTradeStatusRequest) error {
	query := `
		UPDATE trades 
		SET status = $1, mt5_ticket = COALESCE($2, mt5_ticket), 
		    mt5_response = COALESCE($3, mt5_response),
		    entry_price = COALESCE($4, entry_price),
		    current_price = COALESCE($5, current_price),
		    profit_loss = COALESCE($6, profit_loss),
		    commission = COALESCE($7, commission),
		    swap = COALESCE($8, swap),
		    updated_at = NOW(),
		    closed_at = CASE WHEN $1 IN ('closed', 'cancelled') THEN NOW() ELSE closed_at END
		WHERE id = $9
	`

	result, err := db.conn.ExecContext(
		ctx, query,
		req.Status, req.MT5Ticket, req.MT5Response, req.EntryPrice,
		req.CurrentPrice, req.ProfitLoss, req.Commission, req.Swap, tradeID,
	)
	if err != nil {
		return fmt.Errorf("failed to update trade status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("trade with ID %d not found", tradeID)
	}

	return nil
}

// GetOpenTrades retrieves all open trades
func (db *DB) GetOpenTrades(ctx context.Context) ([]*Trade, error) {
	query := `
		SELECT id, uuid, signal_id, symbol, order_type, direction, volume, entry_price,
		       current_price, stop_loss, take_profit, status, mt5_ticket, mt5_response,
		       profit_loss, commission, swap, created_at, updated_at, closed_at
		FROM trades 
		WHERE status IN ('pending', 'filled', 'partial')
		ORDER BY created_at ASC
	`

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query open trades: %w", err)
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		trade := &Trade{}
		err := rows.Scan(
			&trade.ID, &trade.UUID, &trade.SignalID, &trade.Symbol, &trade.OrderType,
			&trade.Direction, &trade.Volume, &trade.EntryPrice, &trade.CurrentPrice,
			&trade.StopLoss, &trade.TakeProfit, &trade.Status, &trade.MT5Ticket,
			&trade.MT5Response, &trade.ProfitLoss, &trade.Commission, &trade.Swap,
			&trade.CreatedAt, &trade.UpdatedAt, &trade.ClosedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade: %w", err)
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// LogEvent logs a system event
func (db *DB) LogEvent(ctx context.Context, level, message, component string, context json.RawMessage) error {
	query := `
		INSERT INTO system_logs (level, message, component, context)
		VALUES ($1, $2, $3, $4)
	`

	_, err := db.conn.ExecContext(ctx, query, level, message, component, context)
	if err != nil {
		return fmt.Errorf("failed to log event: %w", err)
	}

	return nil
} 