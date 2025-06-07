package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"trading-system/internal/config"
	"trading-system/internal/database"
	"trading-system/internal/mt5"
)

// Processor handles signal processing and trade execution
type Processor struct {
	db        *database.DB
	config    *config.Config
	mt5Client *mt5.Client
}

// New creates a new signal processor
func New(db *database.DB, cfg *config.Config) *Processor {
	return &Processor{
		db:        db,
		config:    cfg,
		mt5Client: mt5.NewClient(&cfg.MT5),
	}
}

// GetMT5Client returns the MT5 client for external use
func (p *Processor) GetMT5Client() *mt5.Client {
	return p.mt5Client
}

// Start begins the signal processing loop and position monitoring
func (p *Processor) Start(ctx context.Context) {
	log.Println("Starting signal processor...")

	// Check MT5 connection
	if p.mt5Client.IsConnected(ctx) {
		log.Println("✅ MT5 bridge connection established")
	} else {
		log.Println("⚠️ MT5 bridge not available - trades will be queued")
	}

	// Start signal processing goroutine
	go p.signalProcessingLoop(ctx)
	
	// Start position monitoring goroutine
	go p.positionMonitoringLoop(ctx)

	// Keep the processor running
	<-ctx.Done()
	log.Println("Signal processor stopped")
}

// signalProcessingLoop processes signals every 2 seconds
func (p *Processor) signalProcessingLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.processUnprocessedSignals(ctx); err != nil {
				log.Printf("Error processing signals: %v", err)
			}
		}
	}
}

// positionMonitoringLoop monitors and updates position status every 10 seconds  
func (p *Processor) positionMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.syncPositionsFromMT5(ctx); err != nil {
				log.Printf("Error syncing positions: %v", err)
			}
		}
	}
}

// processUnprocessedSignals retrieves and processes all unprocessed signals
func (p *Processor) processUnprocessedSignals(ctx context.Context) error {
	signals, err := p.db.GetUnprocessedSignals(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed signals: %w", err)
	}

	if len(signals) == 0 {
		return nil // No signals to process
	}

	log.Printf("Processing %d unprocessed signals", len(signals))

	for _, signal := range signals {
		if err := p.processSignal(ctx, signal); err != nil {
			log.Printf("Error processing signal %d: %v", signal.ID, err)
			continue
		}

		// Mark signal as processed
		if err := p.db.MarkSignalProcessed(ctx, signal.ID); err != nil {
			log.Printf("Error marking signal %d as processed: %v", signal.ID, err)
		}
	}

	return nil
}

// processSignal processes a single signal and creates a trade if appropriate
func (p *Processor) processSignal(ctx context.Context, signal *database.Signal) error {
	log.Printf("Processing signal %d: %s %s on %s", signal.ID, signal.SignalType, signal.Symbol, signal.Source)

	// Parse TradingView webhook if applicable
	if signal.Source == "tradingview" {
		return p.processTradingViewSignal(ctx, signal)
	}

	// Handle other signal sources here
	return fmt.Errorf("unsupported signal source: %s", signal.Source)
}

// processTradingViewSignal processes a signal from TradingView
func (p *Processor) processTradingViewSignal(ctx context.Context, signal *database.Signal) error {
	var tvWebhook database.TradingViewWebhook
	if err := json.Unmarshal(signal.Payload, &tvWebhook); err != nil {
		return fmt.Errorf("failed to parse TradingView webhook: %w", err)
	}

	// Validate signal type
	if signal.SignalType == "close" {
		return p.handleCloseSignal(ctx, signal, &tvWebhook)
	}

	// Create trade request from signal
	tradeReq := &database.CreateTradeRequest{
		SignalID:  &signal.ID,
		Symbol:    signal.Symbol,
		OrderType: "market", // Default to market orders for now
		Direction: signal.SignalType,
		Volume:    p.calculatePositionSize(signal.Symbol, tvWebhook.Volume),
		StopLoss:  signal.StopLoss,
		TakeProfit: signal.TakeProfit,
	}

	// Apply risk management checks
	if p.config.Risk.EnableRiskChecks {
		if err := p.validateRiskParameters(ctx, tradeReq); err != nil {
			return fmt.Errorf("risk validation failed: %w", err)
		}
	}

	// Create trade in database
	trade, err := p.db.CreateTrade(ctx, tradeReq)
	if err != nil {
		return fmt.Errorf("failed to create trade: %w", err)
	}

	log.Printf("Created trade %d for signal %d: %s %s %.2f lots", 
		trade.ID, signal.ID, trade.Direction, trade.Symbol, trade.Volume)

	// Execute trade via MT5
	if err := p.executeTrade(ctx, trade); err != nil {
		log.Printf("Failed to execute trade %d: %v", trade.ID, err)
		// Update trade status to failed
		updateReq := &database.UpdateTradeStatusRequest{
			Status: "rejected",
		}
		if updateErr := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); updateErr != nil {
			log.Printf("Failed to update trade %d status: %v", trade.ID, updateErr)
		}
		return err
	}

	return nil
}

// executeTrade sends the trade to MT5 for execution
func (p *Processor) executeTrade(ctx context.Context, trade *database.Trade) error {
	// Check if MT5 is connected
	if !p.mt5Client.IsConnected(ctx) {
		return fmt.Errorf("MT5 bridge not available")
	}

	// Prepare MT5 trade request
	mt5Req := &mt5.TradeRequest{
		Symbol:     trade.Symbol,
		Action:     trade.Direction,
		Volume:     trade.Volume,
		OrderType:  trade.OrderType,
		Comment:    fmt.Sprintf("Signal-%d", *trade.SignalID),
		Magic:      123456, // Fixed magic number for now
	}

	// Add stop loss and take profit if specified
	if trade.StopLoss != nil {
		mt5Req.StopLoss = *trade.StopLoss
	}
	if trade.TakeProfit != nil {
		mt5Req.TakeProfit = *trade.TakeProfit
	}

	// Send trade to MT5
	log.Printf("Sending trade %d to MT5: %s %s %.2f lots", trade.ID, trade.Direction, trade.Symbol, trade.Volume)
	
	mt5Resp, err := p.mt5Client.SendTrade(ctx, mt5Req)
	if err != nil {
		return fmt.Errorf("MT5 trade execution failed: %w", err)
	}

	// Update trade with MT5 response
	mt5ResponseJSON, _ := json.Marshal(mt5Resp)
	
	updateReq := &database.UpdateTradeStatusRequest{
		MT5Response: mt5ResponseJSON,
	}

	if mt5Resp.Success {
		updateReq.Status = "filled"
		updateReq.MT5Ticket = &mt5Resp.Ticket
		updateReq.EntryPrice = &mt5Resp.Price
		if mt5Resp.Commission != 0 {
			updateReq.Commission = &mt5Resp.Commission
		}
		
		log.Printf("✅ Trade %d executed successfully - MT5 ticket: %d, price: %.5f", 
			trade.ID, mt5Resp.Ticket, mt5Resp.Price)
	} else {
		updateReq.Status = "rejected"
		log.Printf("❌ Trade %d rejected by MT5: %s (code: %d)", 
			trade.ID, mt5Resp.ErrorMsg, mt5Resp.ErrorCode)
	}

	return p.db.UpdateTradeStatus(ctx, trade.ID, updateReq)
}

// handleCloseSignal processes a close signal to close existing positions
func (p *Processor) handleCloseSignal(ctx context.Context, signal *database.Signal, tvWebhook *database.TradingViewWebhook) error {
	// Get open trades for this symbol
	openTrades, err := p.db.GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get open trades: %w", err)
	}

	closedCount := 0
	for _, trade := range openTrades {
		if trade.Symbol == signal.Symbol && trade.MT5Ticket != nil {
			// Close position via MT5
			if p.mt5Client.IsConnected(ctx) {
				mt5Resp, err := p.mt5Client.ClosePosition(ctx, *trade.MT5Ticket)
				if err != nil {
					log.Printf("Error closing MT5 position %d: %v", *trade.MT5Ticket, err)
					continue
				}

				// Update trade status
				mt5ResponseJSON, _ := json.Marshal(mt5Resp)
				updateReq := &database.UpdateTradeStatusRequest{
					Status:      "closed",
					MT5Response: mt5ResponseJSON,
				}
				
				if mt5Resp.Success {
					updateReq.ProfitLoss = &mt5Resp.Profit
					if mt5Resp.Commission != 0 {
						updateReq.Commission = &mt5Resp.Commission
					}
					if mt5Resp.Swap != 0 {
						updateReq.Swap = &mt5Resp.Swap
					}
					
					log.Printf("✅ Closed trade %d (MT5 ticket %d) - P&L: %.2f", 
						trade.ID, *trade.MT5Ticket, mt5Resp.Profit)
				} else {
					log.Printf("❌ Failed to close trade %d: %s", trade.ID, mt5Resp.ErrorMsg)
				}
				
				if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
					log.Printf("Error updating trade %d status: %v", trade.ID, err)
				}
			} else {
				// MT5 not available, just mark as closed in database
				updateReq := &database.UpdateTradeStatusRequest{
					Status: "closed",
				}
				if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
					log.Printf("Error updating trade %d status: %v", trade.ID, err)
					continue
				}
				log.Printf("⚠️ Marked trade %d as closed (MT5 offline)", trade.ID)
			}
			
			closedCount++
		}
	}

	log.Printf("Processed close signal for %s: %d trades affected", signal.Symbol, closedCount)
	return nil
}

// syncPositionsFromMT5 synchronizes position data from MT5
func (p *Processor) syncPositionsFromMT5(ctx context.Context) error {
	if !p.mt5Client.IsConnected(ctx) {
		return nil // Skip if MT5 not available
	}

	// Get positions from MT5
	mt5Positions, err := p.mt5Client.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get MT5 positions: %w", err)
	}

	// Get open trades from database
	dbTrades, err := p.db.GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get database trades: %w", err)
	}

	// Update trades with current MT5 position data
	for _, trade := range dbTrades {
		if trade.MT5Ticket == nil {
			continue
		}

		// Find corresponding MT5 position
		var mt5Pos *mt5.PositionInfo
		for _, pos := range mt5Positions {
			if pos.Ticket == *trade.MT5Ticket {
				mt5Pos = pos
				break
			}
		}

		if mt5Pos != nil {
			// Position still exists - update current data
			updateReq := &database.UpdateTradeStatusRequest{
				Status:       "filled", // Ensure it's marked as filled
				CurrentPrice: &mt5Pos.CurrentPrice,
				ProfitLoss:   &mt5Pos.Profit,
				Commission:   &mt5Pos.Commission,
				Swap:         &mt5Pos.Swap,
			}
			
			if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
				log.Printf("Error updating trade %d from MT5 sync: %v", trade.ID, err)
			}
		} else {
			// Position no longer exists in MT5 - mark as closed
			updateReq := &database.UpdateTradeStatusRequest{
				Status: "closed",
			}
			
			if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
				log.Printf("Error marking trade %d as closed: %v", trade.ID, err)
			} else {
				log.Printf("🔄 Synced: Trade %d closed in MT5", trade.ID)
			}
		}
	}

	if len(mt5Positions) > 0 {
		log.Printf("🔄 Position sync complete: %d MT5 positions, %d database trades", 
			len(mt5Positions), len(dbTrades))
	}

	return nil
}

// calculatePositionSize calculates the appropriate position size based on risk parameters
func (p *Processor) calculatePositionSize(symbol string, requestedVolume float64) float64 {
	// If specific volume is requested, use it (but cap it to max position size)
	if requestedVolume > 0 {
		if requestedVolume > p.config.Risk.MaxPositionSize {
			return p.config.Risk.MaxPositionSize
		}
		return requestedVolume
	}

	// Default position size (could be made dynamic based on account balance, volatility, etc.)
	return 0.01 // 0.01 lots = 1000 units for forex
}

// validateRiskParameters validates trade parameters against risk management rules
func (p *Processor) validateRiskParameters(ctx context.Context, tradeReq *database.CreateTradeRequest) error {
	// Check position size
	if tradeReq.Volume > p.config.Risk.MaxPositionSize {
		return fmt.Errorf("position size %.2f exceeds maximum allowed %.2f", 
			tradeReq.Volume, p.config.Risk.MaxPositionSize)
	}

	// Check number of open positions
	openTrades, err := p.db.GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get open trades for risk check: %w", err)
	}

	if len(openTrades) >= p.config.Risk.MaxOpenPositions {
		return fmt.Errorf("maximum open positions reached (%d)", p.config.Risk.MaxOpenPositions)
	}

	// TODO: Add more risk checks:
	// - Daily loss limit
	// - Correlation checks
	// - Account balance checks
	// - Symbol-specific limits

	return nil
}

// ProcessWebhook processes a webhook payload and creates a signal
func (p *Processor) ProcessWebhook(ctx context.Context, webhookData []byte, source string) (*database.Signal, error) {
	// Parse webhook based on source
	var createReq *database.CreateSignalRequest
	var err error

	switch source {
	case "tradingview":
		createReq, err = p.parseTradingViewWebhook(webhookData)
	default:
		return nil, fmt.Errorf("unsupported webhook source: %s", source)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	// Create signal in database
	signal, err := p.db.CreateSignal(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create signal: %w", err)
	}

	log.Printf("Created signal %d: %s %s from %s", signal.ID, signal.SignalType, signal.Symbol, signal.Source)
	return signal, nil
}

// parseTradingViewWebhook parses a TradingView webhook payload
func (p *Processor) parseTradingViewWebhook(data []byte) (*database.CreateSignalRequest, error) {
	var webhook database.TradingViewWebhook
	if err := json.Unmarshal(data, &webhook); err != nil {
		return nil, fmt.Errorf("failed to parse TradingView webhook JSON: %w", err)
	}

	// Validate required fields
	if webhook.Ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	if webhook.Action == "" {
		return nil, fmt.Errorf("action is required")
	}

	// Map action to signal type
	signalType := webhook.Action
	if signalType != "buy" && signalType != "sell" && signalType != "close" {
		return nil, fmt.Errorf("invalid action: %s, must be buy/sell/close", webhook.Action)
	}

	// Create signal request
	req := &database.CreateSignalRequest{
		Source:     "tradingview",
		Symbol:     webhook.Ticker,
		SignalType: signalType,
		Payload:    data, // Store the raw webhook data
	}

	// Add optional fields
	if webhook.Price > 0 {
		req.Price = &webhook.Price
	}
	if webhook.StopLoss > 0 {
		req.StopLoss = &webhook.StopLoss
	}
	if webhook.TakeProfit > 0 {
		req.TakeProfit = &webhook.TakeProfit
	}

	return req, nil
} 