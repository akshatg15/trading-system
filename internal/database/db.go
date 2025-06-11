package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
		INSERT INTO signals (source, symbol, signal_type, price, stop_loss, take_profit, tp1, tp2, sl1, sl2, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, uuid, source, symbol, signal_type, price, stop_loss, take_profit, tp1, tp2, sl1, sl2,
		          payload, processed, processed_at, created_at, updated_at
	`

	signal := &Signal{}
	err := db.conn.QueryRowContext(
		ctx, query,
		req.Source, req.Symbol, req.SignalType, req.Price, req.StopLoss, req.TakeProfit, req.TP1, req.TP2, req.SL1, req.SL2, req.Payload,
	).Scan(
		&signal.ID, &signal.UUID, &signal.Source, &signal.Symbol, &signal.SignalType,
		&signal.Price, &signal.StopLoss, &signal.TakeProfit, &signal.TP1, &signal.TP2, &signal.SL1, &signal.SL2, &signal.Payload,
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
		SELECT id, uuid, source, symbol, signal_type, price, stop_loss, take_profit, tp1, tp2, sl1, sl2,
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
			&signal.Price, &signal.StopLoss, &signal.TakeProfit, &signal.TP1, &signal.TP2, &signal.SL1, &signal.SL2, &signal.Payload,
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
		INSERT INTO trades (signal_id, parent_signal_id, parent_trade_id, trade_type, symbol, order_type, direction, volume, entry_price, stop_loss, take_profit, tp1, tp2, sl1, sl2)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, uuid, signal_id, parent_signal_id, parent_trade_id, trade_type, symbol, order_type, direction, volume, entry_price,
		          current_price, stop_loss, take_profit, tp1, tp2, sl1, sl2, status, mt5_ticket, mt5_response,
		          profit_loss, commission, swap, created_at, updated_at, closed_at
	`

	trade := &Trade{}
	err := db.conn.QueryRowContext(
		ctx, query,
		req.SignalID, req.ParentSignalID, req.ParentTradeID, req.TradeType, req.Symbol, req.OrderType, req.Direction, req.Volume,
		req.EntryPrice, req.StopLoss, req.TakeProfit, req.TP1, req.TP2, req.SL1, req.SL2,
	).Scan(
		&trade.ID, &trade.UUID, &trade.SignalID, &trade.ParentSignalID, &trade.ParentTradeID, &trade.TradeType, &trade.Symbol, &trade.OrderType,
		&trade.Direction, &trade.Volume, &trade.EntryPrice, &trade.CurrentPrice,
		&trade.StopLoss, &trade.TakeProfit, &trade.TP1, &trade.TP2, &trade.SL1, &trade.SL2, &trade.Status, &trade.MT5Ticket,
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

	// Build dynamic query based on what fields are provided
	var setParts []string
	var args []interface{}
	argIndex := 1

	// Status is always required
	if req.Status == "" {
		return fmt.Errorf("status is required")
	}
	setParts = append(setParts, fmt.Sprintf("status = $%d", argIndex))
	args = append(args, req.Status)
	argIndex++

	// Add optional fields only if they're provided
	if req.MT5Ticket != nil {
		setParts = append(setParts, fmt.Sprintf("mt5_ticket = $%d", argIndex))
		args = append(args, *req.MT5Ticket)
		argIndex++
	}

	if req.MT5Response != nil {
		setParts = append(setParts, fmt.Sprintf("mt5_response = $%d", argIndex))
		args = append(args, *req.MT5Response)
		argIndex++
	}

	if req.EntryPrice != nil {
		setParts = append(setParts, fmt.Sprintf("entry_price = $%d", argIndex))
		args = append(args, *req.EntryPrice)
		argIndex++
	}

	if req.CurrentPrice != nil {
		setParts = append(setParts, fmt.Sprintf("current_price = $%d", argIndex))
		args = append(args, *req.CurrentPrice)
		argIndex++
	}

	if req.ProfitLoss != nil {
		setParts = append(setParts, fmt.Sprintf("profit_loss = $%d", argIndex))
		args = append(args, *req.ProfitLoss)
		argIndex++
	}

	if req.Commission != nil {
		setParts = append(setParts, fmt.Sprintf("commission = $%d", argIndex))
		args = append(args, *req.Commission)
		argIndex++
	}

	if req.Swap != nil {
		setParts = append(setParts, fmt.Sprintf("swap = $%d", argIndex))
		args = append(args, *req.Swap)
		argIndex++
	}

	// Always update updated_at
	setParts = append(setParts, "updated_at = NOW()")

	// Handle closed_at based on status
	if req.Status == "closed" || req.Status == "cancelled" {
		setParts = append(setParts, "closed_at = NOW()")
	}

	// Build final query
	query := fmt.Sprintf(`
		UPDATE trades 
		SET %s
		WHERE id = $%d
	`, strings.Join(setParts, ", "), argIndex)

	args = append(args, tradeID)

	result, err := db.conn.ExecContext(ctx, query, args...)
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
		SELECT id, uuid, signal_id, parent_signal_id, parent_trade_id, trade_type, symbol, order_type, direction, volume, entry_price,
		       current_price, stop_loss, take_profit, tp1, tp2, sl1, sl2, status, mt5_ticket, mt5_response,
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
			&trade.ID, &trade.UUID, &trade.SignalID, &trade.ParentSignalID, &trade.ParentTradeID, &trade.TradeType, &trade.Symbol, &trade.OrderType,
			&trade.Direction, &trade.Volume, &trade.EntryPrice, &trade.CurrentPrice,
			&trade.StopLoss, &trade.TakeProfit, &trade.TP1, &trade.TP2, &trade.SL1, &trade.SL2, &trade.Status, &trade.MT5Ticket,
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

// GetTradesByParent retrieves all child trades for a given parent trade ID
func (db *DB) GetTradesByParent(ctx context.Context, parentTradeID int) ([]*Trade, error) {
	query := `
		SELECT id, uuid, signal_id, parent_signal_id, parent_trade_id, trade_type, symbol, order_type, direction, volume, entry_price,
		       current_price, stop_loss, take_profit, tp1, tp2, sl1, sl2, status, mt5_ticket, mt5_response,
		       profit_loss, commission, swap, created_at, updated_at, closed_at
		FROM trades 
		WHERE parent_trade_id = $1
		ORDER BY created_at ASC
	`

	rows, err := db.conn.QueryContext(ctx, query, parentTradeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query child trades: %w", err)
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		trade := &Trade{}
		err := rows.Scan(
			&trade.ID, &trade.UUID, &trade.SignalID, &trade.ParentSignalID, &trade.ParentTradeID, &trade.TradeType, &trade.Symbol, &trade.OrderType,
			&trade.Direction, &trade.Volume, &trade.EntryPrice, &trade.CurrentPrice,
			&trade.StopLoss, &trade.TakeProfit, &trade.TP1, &trade.TP2, &trade.SL1, &trade.SL2, &trade.Status, &trade.MT5Ticket,
			&trade.MT5Response, &trade.ProfitLoss, &trade.Commission, &trade.Swap,
			&trade.CreatedAt, &trade.UpdatedAt, &trade.ClosedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan child trade: %w", err)
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
