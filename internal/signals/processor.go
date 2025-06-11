package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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
		processErr := p.processSignal(ctx, signal)
		if processErr != nil {
			log.Printf("Error processing signal %d: %v", signal.ID, processErr)
		}

		// Always mark signal as processed after attempting to handle it
		// This prevents infinite loops when MT5 is unavailable
		if err := p.db.MarkSignalProcessed(ctx, signal.ID); err != nil {
			log.Printf("Error marking signal %d as processed: %v", signal.ID, err)
		} else {
			if processErr != nil {
				log.Printf("Signal %d marked as processed despite processing error: %v", signal.ID, processErr)
			} else {
				log.Printf("Signal %d successfully processed and marked as completed", signal.ID)
			}
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

	// Check MT5 connectivity first - don't create trades if not connected
	if !p.mt5Client.IsConnected(ctx) {
		log.Printf("MT5 bridge not available - rejecting signal %d", signal.ID)
		return fmt.Errorf("MT5 bridge not available - cannot execute trades")
	}

	// Get volume value (handle pointer)
	requestedVolume := 0.0
	if tvWebhook.Volume != nil {
		requestedVolume = *tvWebhook.Volume
	}

	// Apply risk management checks first
	if p.config.Risk.EnableRiskChecks {
		if err := p.validateRiskParametersFromSignal(ctx, signal, requestedVolume); err != nil {
			return fmt.Errorf("risk validation failed: %w", err)
		}
	}

	// Calculate position size
	totalVolume := p.calculatePositionSize(signal.Symbol, requestedVolume)

	// Create entry trade
	entryTrade, err := p.createEntryTrade(ctx, signal, totalVolume)
	if err != nil {
		return fmt.Errorf("failed to create entry trade: %w", err)
	}

	log.Printf("Created entry trade %d for signal %d: %s %s %.2f lots",
		entryTrade.ID, signal.ID, entryTrade.Direction, entryTrade.Symbol, entryTrade.Volume)

	// Execute entry trade via MT5 (no TP levels, just entry with SL)
	if err := p.executeEntryTrade(ctx, entryTrade); err != nil {
		log.Printf("Failed to execute entry trade %d: %v", entryTrade.ID, err)
		p.updateTradeStatus(ctx, entryTrade.ID, "rejected", nil)
		log.Printf("Trade %d marked as rejected due to execution failure", entryTrade.ID)
		return fmt.Errorf("entry trade execution failed: %w", err)
	}

	// Only proceed with TP orders if entry was successful
	log.Printf("Entry trade %d successfully executed, creating TP orders...", entryTrade.ID)

	// Add a small delay to ensure MT5 processes the entry trade before creating TP orders
	time.Sleep(500 * time.Millisecond)

	// Create and execute TP1 order if available
	if signal.TP1 != nil && *signal.TP1 > 0 {
		tp1Volume := totalVolume / 2 // 50% for TP1
		tp1Trade, err := p.createTPTradeWithRetry(ctx, signal, entryTrade.ID, "tp1", *signal.TP1, tp1Volume)
		if err != nil {
			log.Printf("Failed to create TP1 trade: %v", err)
		} else {
			log.Printf("Created TP1 trade %d for signal %d", tp1Trade.ID, signal.ID)

			// Execute TP1 limit order
			if err := p.executeTPTrade(ctx, tp1Trade); err != nil {
				log.Printf("Failed to execute TP1 trade %d: %v", tp1Trade.ID, err)
				p.updateTradeStatus(ctx, tp1Trade.ID, "rejected", nil)
			} else {
				log.Printf("TP1 trade %d successfully placed", tp1Trade.ID)
			}
		}
	}

	// Create and execute TP2 order if available
	if signal.TP2 != nil && *signal.TP2 > 0 {
		tp2Volume := totalVolume / 2 // 50% for TP2
		tp2Trade, err := p.createTPTradeWithRetry(ctx, signal, entryTrade.ID, "tp2", *signal.TP2, tp2Volume)
		if err != nil {
			log.Printf("Failed to create TP2 trade: %v", err)
		} else {
			log.Printf("Created TP2 trade %d for signal %d", tp2Trade.ID, signal.ID)

			// Execute TP2 limit order
			if err := p.executeTPTrade(ctx, tp2Trade); err != nil {
				log.Printf("Failed to execute TP2 trade %d: %v", tp2Trade.ID, err)
				p.updateTradeStatus(ctx, tp2Trade.ID, "rejected", nil)
			} else {
				log.Printf("TP2 trade %d successfully placed", tp2Trade.ID)
			}
		}
	}

	return nil
}

// createEntryTrade creates the main entry trade
func (p *Processor) createEntryTrade(ctx context.Context, signal *database.Signal, volume float64) (*database.Trade, error) {
	tradeReq := &database.CreateTradeRequest{
		SignalID:       &signal.ID,
		ParentSignalID: &signal.ID,
		TradeType:      "entry",
		Symbol:         signal.Symbol,
		OrderType:      "market",
		Direction:      signal.SignalType,
		Volume:         volume,
		EntryPrice:     signal.Price,
		StopLoss:       signal.StopLoss,
		TakeProfit:     signal.TakeProfit,
		TP1:            signal.TP1,
		TP2:            signal.TP2,
		SL1:            signal.SL1,
		SL2:            signal.SL2,
	}

	return p.db.CreateTrade(ctx, tradeReq)
}

// createTPTrade creates a take profit trade
func (p *Processor) createTPTrade(ctx context.Context, signal *database.Signal, parentTradeID int, tpType string, tpPrice float64, volume float64) (*database.Trade, error) {
	tradeReq := &database.CreateTradeRequest{
		SignalID:       &signal.ID,
		ParentSignalID: &signal.ID,     // Reference the original signal
		ParentTradeID:  &parentTradeID, // Reference the parent entry trade
		TradeType:      tpType,
		Symbol:         signal.Symbol,
		OrderType:      "limit",
		Direction:      getOppositeDirection(signal.SignalType),
		Volume:         volume,
		EntryPrice:     &tpPrice,
		StopLoss:       signal.StopLoss,
		TakeProfit:     &tpPrice,
		TP1:            signal.TP1,
		TP2:            signal.TP2,
		SL1:            signal.SL1,
		SL2:            signal.SL2,
	}

	return p.db.CreateTrade(ctx, tradeReq)
}

// getOppositeDirection returns the opposite direction for closing trades
func getOppositeDirection(direction string) string {
	if direction == "buy" {
		return "sell"
	}
	return "buy"
}

// updateTradeStatus is a helper function to update trade status
func (p *Processor) updateTradeStatus(ctx context.Context, tradeID int, status string, mt5Response *json.RawMessage) {
	updateReq := &database.UpdateTradeStatusRequest{
		Status:      status,
		MT5Response: mt5Response,
	}
	if err := p.db.UpdateTradeStatus(ctx, tradeID, updateReq); err != nil {
		log.Printf("Failed to update trade %d status: %v", tradeID, err)
	}
}

// executeEntryTrade sends the entry trade to MT5 for execution (no TP levels)
func (p *Processor) executeEntryTrade(ctx context.Context, trade *database.Trade) error {
	// Check if MT5 is connected
	if !p.mt5Client.IsConnected(ctx) {
		return fmt.Errorf("MT5 bridge not available")
	}

	// Prepare MT5 trade request for entry only
	mt5Req := &mt5.TradeRequest{
		Symbol:    trade.Symbol,
		Action:    trade.Direction,
		Volume:    trade.Volume,
		OrderType: trade.OrderType,
	}

	// Add optional price fields
	if trade.EntryPrice != nil {
		mt5Req.Price = *trade.EntryPrice
	}
	if trade.StopLoss != nil {
		mt5Req.StopLoss = *trade.StopLoss
	}
	// Note: No TakeProfit, TP1, TP2 - these will be separate orders

	log.Printf("Sending entry trade to MT5: Symbol=%s, Action=%s, Volume=%.2f, Price=%.5f, SL=%.5f",
		mt5Req.Symbol, mt5Req.Action, mt5Req.Volume, mt5Req.Price, mt5Req.StopLoss)

	// Send trade to MT5
	response, err := p.mt5Client.SendTrade(ctx, mt5Req)
	if err != nil {
		return fmt.Errorf("failed to send entry trade to MT5: %w", err)
	}

	// Update trade with MT5 response
	responseData, _ := json.Marshal(response)
	responseRaw := json.RawMessage(responseData)

	updateReq := &database.UpdateTradeStatusRequest{
		MT5Response: &responseRaw,
	}

	if response.Success {
		updateReq.Status = "filled"

		// Simple single ticket response for entry trades
		if response.Ticket != 0 {
			updateReq.MT5Ticket = &response.Ticket
		}
		updateReq.EntryPrice = &response.Price

		if response.Commission != 0 {
			updateReq.Commission = &response.Commission
		}
	} else {
		updateReq.Status = "rejected"
	}

	return p.db.UpdateTradeStatus(ctx, trade.ID, updateReq)
}

// executeTPTrade sends a TP trade to MT5 for execution as a position-based closing order
func (p *Processor) executeTPTrade(ctx context.Context, trade *database.Trade) error {
	// Check if MT5 is connected
	if !p.mt5Client.IsConnected(ctx) {
		return fmt.Errorf("MT5 bridge not available")
	}

	// Get the parent entry trade to find the MT5 position ticket
	if trade.ParentTradeID == nil {
		return fmt.Errorf("TP trade missing parent trade ID")
	}

	parentTrade, err := p.getTradeByID(ctx, *trade.ParentTradeID)
	if err != nil {
		return fmt.Errorf("failed to get parent trade: %w", err)
	}

	if parentTrade.MT5Ticket == nil {
		return fmt.Errorf("parent trade missing MT5 ticket")
	}

	// Verify the parent position still exists
	if !p.verifyPositionExists(ctx, *parentTrade.MT5Ticket) {
		log.Printf("Parent position %d no longer exists, skipping TP order", *parentTrade.MT5Ticket)
		p.updateTradeStatus(ctx, trade.ID, "cancelled", nil)
		return nil
	}

	// Create position-based TP order using MT5 PositionModify to set TP level
	mt5Req := &mt5.PositionModifyRequest{
		PositionTicket: *parentTrade.MT5Ticket,
		Symbol:         trade.Symbol,
		TakeProfit:     *trade.EntryPrice, // TP price level
		StopLoss:       trade.StopLoss,    // Keep existing SL if any
		PartialVolume:  trade.Volume,      // Volume to close at this level
		TPType:         trade.TradeType,   // "tp1" or "tp2"
	}

	log.Printf("Setting TP level for position %d: Symbol=%s, TP=%.5f, Volume=%.2f, Type=%s",
		*parentTrade.MT5Ticket, mt5Req.Symbol, mt5Req.TakeProfit, mt5Req.PartialVolume, mt5Req.TPType)

	// Send TP modification request to MT5
	response, err := p.mt5Client.ModifyPosition(ctx, mt5Req)
	if err != nil {
		return fmt.Errorf("failed to set TP level in MT5: %w", err)
	}

	// Update trade with MT5 response
	responseData, _ := json.Marshal(response)
	responseRaw := json.RawMessage(responseData)

	updateReq := &database.UpdateTradeStatusRequest{
		MT5Response: &responseRaw,
	}

	if response.Success {
		updateReq.Status = "pending" // TP level set, waiting for price to be hit

		// Use the parent position ticket as reference
		updateReq.MT5Ticket = parentTrade.MT5Ticket
		
		// Store the TP level ticket if returned
		if response.TPOrderTicket != 0 {
			// Create a JSON object to store TP order details
			tpDetails := map[string]interface{}{
				"tp_order_ticket": response.TPOrderTicket,
				"parent_ticket":   *parentTrade.MT5Ticket,
				"tp_price":        *trade.EntryPrice,
				"tp_volume":       trade.Volume,
			}
			tpDetailsData, _ := json.Marshal(tpDetails)
			tpDetailsRaw := json.RawMessage(tpDetailsData)
			updateReq.MT5Response = &tpDetailsRaw
		}

		if response.Commission != 0 {
			updateReq.Commission = &response.Commission
		}

		log.Printf("TP level %s successfully set for position %d", trade.TradeType, *parentTrade.MT5Ticket)
	} else {
		updateReq.Status = "rejected"
		log.Printf("Failed to set TP level %s for position %d: %s", trade.TradeType, *parentTrade.MT5Ticket, response.ErrorMsg)
	}

	return p.db.UpdateTradeStatus(ctx, trade.ID, updateReq)
}

// handleCloseSignal handles close signals by closing all open positions for a symbol
func (p *Processor) handleCloseSignal(ctx context.Context, signal *database.Signal, tvWebhook *database.TradingViewWebhook) error {
	log.Printf("Processing close signal for %s", signal.Symbol)

	// Get all open trades for this symbol
	openTrades, err := p.db.GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get open trades: %w", err)
	}

	var tradesToClose []*database.Trade
	for _, trade := range openTrades {
		if trade.Symbol == signal.Symbol && trade.Status == "filled" {
			tradesToClose = append(tradesToClose, trade)
		}
	}

	if len(tradesToClose) == 0 {
		log.Printf("No open trades found for %s", signal.Symbol)
		return nil
	}

	log.Printf("Closing %d open trades for %s", len(tradesToClose), signal.Symbol)

	// Close each trade via MT5
	for _, trade := range tradesToClose {
		if err := p.closeTradeInMT5(ctx, trade); err != nil {
			log.Printf("Failed to close trade %d: %v", trade.ID, err)
			continue
		}

		// Update trade status to closed
		p.updateTradeStatus(ctx, trade.ID, "closed", nil)
	}

	return nil
}

// closeTradeInMT5 closes a specific trade in MT5
func (p *Processor) closeTradeInMT5(ctx context.Context, trade *database.Trade) error {
	if !p.mt5Client.IsConnected(ctx) {
		return fmt.Errorf("MT5 bridge not available")
	}

	// Send close request to MT5
	response, err := p.mt5Client.ClosePosition(ctx, *trade.MT5Ticket)
	if err != nil {
		return fmt.Errorf("failed to close position in MT5: %w", err)
	}

	log.Printf("Closed trade %d (MT5 ticket %d) with result: %v", trade.ID, *trade.MT5Ticket, response)
	return nil
}

// syncPositionsFromMT5 synchronizes position data from MT5
func (p *Processor) syncPositionsFromMT5(ctx context.Context) error {
	if !p.mt5Client.IsConnected(ctx) {
		return nil // Skip if MT5 not available
	}

	// Get open trades from database
	openTrades, err := p.db.GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get open trades: %w", err)
	}

	// Get both positions and pending orders from MT5
	positions, err := p.mt5Client.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get positions from MT5: %w", err)
	}

	orders, err := p.mt5Client.GetOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get orders from MT5: %w", err)
	}

	// Create maps for quick lookup - positions are executed trades
	mt5Tickets := make(map[int64]*mt5.PositionInfo)
	for _, pos := range positions {
		mt5Tickets[pos.Ticket] = pos
	}

	// Create maps for pending orders
	mt5OrderTickets := make(map[int64]*mt5.OrderInfo)
	for _, order := range orders {
		mt5OrderTickets[order.Ticket] = order
	}

	// Update trades with current MT5 data
	for _, trade := range openTrades {
		if trade.MT5Ticket == nil {
			continue
		}

		// Check if this is a position (executed trade)
		if pos, exists := mt5Tickets[*trade.MT5Ticket]; exists {
			// Position still exists in MT5, update current data
			updateReq := &database.UpdateTradeStatusRequest{
				Status:       "filled", // Ensure status is set
				CurrentPrice: &pos.CurrentPrice,
				ProfitLoss:   &pos.Profit,
				Commission:   &pos.Commission,
				Swap:         &pos.Swap,
			}

			if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
				log.Printf("Failed to update trade %d: %v", trade.ID, err)
			}
		} else if order, exists := mt5OrderTickets[*trade.MT5Ticket]; exists {
			// This is a pending order (limit order not yet executed)
			// Keep status as "pending" and update price if needed
			updateReq := &database.UpdateTradeStatusRequest{
				Status: "pending", // Keep as pending
			}

			// Update price if available
			if order.Price > 0 {
				updateReq.EntryPrice = &order.Price
			}

			if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
				log.Printf("Failed to update pending trade %d: %v", trade.ID, err)
			}
		} else {
			// Neither position nor pending order exists - now it's truly closed
			// This could mean: 1) Order was cancelled, 2) Order was filled and position was closed
			// Only mark as closed if it was previously filled, or if it's an older trade
			
			if trade.Status == "filled" {
				// Position was closed
				log.Printf("Trade %d (MT5 ticket %d) position no longer exists in MT5, marking as closed", trade.ID, *trade.MT5Ticket)
				p.updateTradeStatus(ctx, trade.ID, "closed", nil)
			} else if trade.Status == "pending" {
				// Pending order was removed/cancelled
				log.Printf("Trade %d (MT5 ticket %d) pending order no longer exists in MT5, marking as cancelled", trade.ID, *trade.MT5Ticket)
				p.updateTradeStatus(ctx, trade.ID, "cancelled", nil)
			}
		}
	}

	return nil
}

// calculatePositionSize calculates the appropriate position size based on risk management
func (p *Processor) calculatePositionSize(symbol string, requestedVolume float64) float64 {
	// Use requested volume if provided and within limits
	if requestedVolume > 0 && requestedVolume <= p.config.Risk.MaxPositionSize {
		return requestedVolume
	}

	// Default to minimum volume (0.01 lots for personal trading)
	return 0.10
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

// validateRiskParametersFromSignal validates trade parameters against risk management rules
func (p *Processor) validateRiskParametersFromSignal(ctx context.Context, signal *database.Signal, volume float64) error {
	// Check position size
	if volume > p.config.Risk.MaxPositionSize {
		return fmt.Errorf("position size %.2f exceeds maximum allowed %.2f",
			volume, p.config.Risk.MaxPositionSize)
	}

	// Check number of open positions using actual MT5 positions
	if p.mt5Client.IsConnected(ctx) {
		// Use efficient position count endpoint
		positionCount, err := p.mt5Client.GetPositionCount(ctx)
		if err != nil {
			log.Printf("Warning: Failed to get MT5 position count for risk check, falling back to database: %v", err)
			// Fallback to database check
			return p.validateRiskParametersFromDatabase(ctx, volume)
		}

		if positionCount >= p.config.Risk.MaxOpenPositions {
			return fmt.Errorf("maximum open positions reached (%d)", p.config.Risk.MaxOpenPositions)
		}

		log.Printf("Risk check passed: %d/%d positions open", positionCount, p.config.Risk.MaxOpenPositions)
	} else {
		// Fallback to database check if MT5 not available
		log.Printf("Warning: MT5 not connected, using database for risk check")
		return p.validateRiskParametersFromDatabase(ctx, volume)
	}

	// TODO: Add more risk checks:
	// - Daily loss limit
	// - Correlation checks
	// - Account balance checks
	// - Symbol-specific limits

	return nil
}

// validateRiskParametersFromDatabase is a fallback method using database records
func (p *Processor) validateRiskParametersFromDatabase(ctx context.Context, volume float64) error {
	// Check position size
	if volume > p.config.Risk.MaxPositionSize {
		return fmt.Errorf("position size %.2f exceeds maximum allowed %.2f",
			volume, p.config.Risk.MaxPositionSize)
	}

	// Check number of open positions from database
	openTrades, err := p.db.GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get open trades for risk check: %w", err)
	}

	if len(openTrades) >= p.config.Risk.MaxOpenPositions {
		return fmt.Errorf("maximum open positions reached (%d)", p.config.Risk.MaxOpenPositions)
	}

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
	// First try to parse as JSON
	var webhook database.TradingViewWebhook
	if err := json.Unmarshal(data, &webhook); err != nil {
		// If JSON parsing fails, try to parse as simple pipe-delimited format
		// Format: ticker|action|entry|stop_loss|tp1|tp2|volume|timestamp
		return p.parseSimpleFormat(string(data))
	}

	// Continue with existing JSON parsing logic
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

	// Parse timestamp (flexible format)
	timestampStr := p.parseTimestamp(webhook.Timestamp)

	// Clean up the payload to avoid large timestamp values causing DB overflow
	cleanedPayload := data
	if len(webhook.Timestamp) > 0 {
		// Replace the original timestamp with the parsed string version to avoid overflow
		webhookCopy := webhook
		webhookCopy.Timestamp = json.RawMessage(`"` + timestampStr + `"`)
		if cleanedData, err := json.Marshal(webhookCopy); err == nil {
			cleanedPayload = cleanedData
		}
	}

	// Create signal request
	req := &database.CreateSignalRequest{
		Source:     "tradingview",
		Symbol:     webhook.Ticker,
		SignalType: signalType,
		Payload:    cleanedPayload, // Store the cleaned webhook data
	}

	// Helper function to validate and set price fields
	validatePrice := func(price *float64, fieldName string) (*float64, error) {
		if price == nil || *price <= 0 {
			return nil, nil
		}
		// Validate price is within reasonable range for DECIMAL(20,8) - max 12 digits before decimal, 8 after
		// Maximum safe value is 999999999999.99999999
		if *price > 999999999999.99999999 {
			return nil, fmt.Errorf("%s value too large: %.8f (max allowed: 999999999999.99999999)", fieldName, *price)
		}
		// Round to 8 decimal places to match database precision
		rounded := float64(int(*price*100000000+0.5)) / 100000000
		return &rounded, nil
	}

	// Add optional price fields with validation
	var err error

	// Use Entry field if available, otherwise fall back to Price field
	if webhook.Entry != nil {
		if req.Price, err = validatePrice(webhook.Entry, "entry"); err != nil {
			return nil, err
		}
	} else if webhook.Price != nil {
		if req.Price, err = validatePrice(webhook.Price, "price"); err != nil {
			return nil, err
		}
	}

	if req.StopLoss, err = validatePrice(webhook.StopLoss, "stop_loss"); err != nil {
		return nil, err
	}

	if req.TakeProfit, err = validatePrice(webhook.TakeProfit, "take_profit"); err != nil {
		return nil, err
	}

	if req.TP1, err = validatePrice(webhook.TP1, "tp1"); err != nil {
		return nil, err
	}

	if req.TP2, err = validatePrice(webhook.TP2, "tp2"); err != nil {
		return nil, err
	}

	// Validate TP ordering based on signal direction
	if req.TP1 != nil && req.TP2 != nil {
		if signalType == "buy" && *req.TP2 <= *req.TP1 {
			return nil, fmt.Errorf("for buy signals, TP2 (%.5f) must be greater than TP1 (%.5f)", *req.TP2, *req.TP1)
		}
		if signalType == "sell" && *req.TP2 >= *req.TP1 {
			return nil, fmt.Errorf("for sell signals, TP2 (%.5f) must be less than TP1 (%.5f)", *req.TP2, *req.TP1)
		}
	}

	// Log the parsed webhook for debugging
	log.Printf("Parsed webhook: Symbol=%s, Action=%s, Entry=%.5f, SL=%.5f, TP1=%.5f, TP2=%.5f, Timestamp=%s",
		req.Symbol, req.SignalType,
		safeFloatValue(req.Price),
		safeFloatValue(req.StopLoss),
		safeFloatValue(req.TP1),
		safeFloatValue(req.TP2),
		timestampStr)

	// Debug log the actual values being sent to database
	log.Printf("DB values: Price=%v, StopLoss=%v, TP1=%v, TP2=%v",
		req.Price, req.StopLoss, req.TP1, req.TP2)

	return req, nil
}

// parseTimestamp handles flexible timestamp formats (number or string)
func (p *Processor) parseTimestamp(timestampRaw json.RawMessage) string {
	if len(timestampRaw) == 0 {
		return time.Now().Format(time.RFC3339)
	}

	// Try to parse as string first (quoted)
	var timestampStr string
	if err := json.Unmarshal(timestampRaw, &timestampStr); err == nil {
		return timestampStr
	}

	// Try to parse as number (Unix timestamp)
	var timestampNum int64
	if err := json.Unmarshal(timestampRaw, &timestampNum); err == nil {
		// Handle both seconds and milliseconds timestamps
		if timestampNum > 1e12 {
			// If timestamp is larger than 1e12, it's likely in milliseconds
			timestampNum = timestampNum / 1000
		}

		// Validate timestamp is within reasonable range (year 1970-2100)
		if timestampNum < 0 || timestampNum > 4102444800 { // Jan 1, 2100
			log.Printf("Warning: Invalid timestamp %d, using current time", timestampNum)
			return time.Now().Format(time.RFC3339)
		}

		return time.Unix(timestampNum, 0).Format(time.RFC3339)
	}

	// Fallback to current time if parsing fails
	log.Printf("Warning: Could not parse timestamp %s, using current time", string(timestampRaw))
	return time.Now().Format(time.RFC3339)
}

// Helper function to safely get float value for logging
func safeFloatValue(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

// parseSimpleFormat parses pipe-delimited format: ticker|action|entry|stop_loss|tp1|tp2|volume|timestamp
func (p *Processor) parseSimpleFormat(data string) (*database.CreateSignalRequest, error) {
	parts := strings.Split(strings.TrimSpace(data), "|")
	if len(parts) != 8 {
		return nil, fmt.Errorf("simple format requires 8 parts separated by |, got %d parts", len(parts))
	}

	ticker := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	entryStr := strings.TrimSpace(parts[2])
	slStr := strings.TrimSpace(parts[3])
	tp1Str := strings.TrimSpace(parts[4])
	tp2Str := strings.TrimSpace(parts[5])
	volumeStr := strings.TrimSpace(parts[6])
	timestampStr := strings.TrimSpace(parts[7])

	// Validate required fields
	if ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}
	if action != "buy" && action != "sell" && action != "close" {
		return nil, fmt.Errorf("invalid action: %s, must be buy/sell/close", action)
	}

	// Parse numeric values
	parseFloat := func(s, field string) (*float64, error) {
		if s == "" || s == "0" {
			return nil, nil
		}
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %s", field, s)
		}
		if val <= 0 {
			return nil, nil
		}
		// Round to 8 decimal places to match database precision
		rounded := float64(int(val*100000000+0.5)) / 100000000
		return &rounded, nil
	}

	entry, err := parseFloat(entryStr, "entry")
	if err != nil {
		return nil, err
	}

	stopLoss, err := parseFloat(slStr, "stop_loss")
	if err != nil {
		return nil, err
	}

	tp1, err := parseFloat(tp1Str, "tp1")
	if err != nil {
		return nil, err
	}

	tp2, err := parseFloat(tp2Str, "tp2")
	if err != nil {
		return nil, err
	}

	// Validate TP ordering based on signal direction
	if tp1 != nil && tp2 != nil {
		if action == "buy" && *tp2 <= *tp1 {
			return nil, fmt.Errorf("for buy signals, TP2 (%.5f) must be greater than TP1 (%.5f)", *tp2, *tp1)
		}
		if action == "sell" && *tp2 >= *tp1 {
			return nil, fmt.Errorf("for sell signals, TP2 (%.5f) must be less than TP1 (%.5f)", *tp2, *tp1)
		}
	}

	// Create simple payload for storage
	simplePayload := fmt.Sprintf(`{"ticker":"%s","action":"%s","entry":%s,"stop_loss":%s,"tp1":%s,"tp2":%s,"volume":%s,"timestamp":"%s","format":"simple"}`,
		ticker, action, entryStr, slStr, tp1Str, tp2Str, volumeStr, timestampStr)

	req := &database.CreateSignalRequest{
		Source:     "tradingview",
		Symbol:     ticker,
		SignalType: action,
		Price:      entry,
		StopLoss:   stopLoss,
		TP1:        tp1,
		TP2:        tp2,
		Payload:    []byte(simplePayload),
	}

	log.Printf("Parsed simple format: Symbol=%s, Action=%s, Entry=%.5f, SL=%.5f, TP1=%.5f, TP2=%.5f",
		req.Symbol, req.SignalType,
		safeFloatValue(req.Price),
		safeFloatValue(req.StopLoss),
		safeFloatValue(req.TP1),
		safeFloatValue(req.TP2))

	return req, nil
}

// createTPTradeWithRetry creates a TP trade with retry logic to handle database connection issues
func (p *Processor) createTPTradeWithRetry(ctx context.Context, signal *database.Signal, parentTradeID int, tpType string, tpPrice float64, volume float64) (*database.Trade, error) {
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait a bit between retries
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			log.Printf("Retrying TP trade creation, attempt %d/%d", attempt+1, maxRetries)
		}
		
		trade, err := p.createTPTrade(ctx, signal, parentTradeID, tpType, tpPrice, volume)
		if err == nil {
			return trade, nil
		}
		
		lastErr = err
		log.Printf("Failed to create TP trade (attempt %d/%d): %v", attempt+1, maxRetries, err)
	}
	
	return nil, fmt.Errorf("failed to create TP trade after %d attempts: %w", maxRetries, lastErr)
}

// getTradeByID retrieves a trade by its ID
func (p *Processor) getTradeByID(ctx context.Context, tradeID int) (*database.Trade, error) {
	return p.db.GetTradeByID(ctx, tradeID)
}

// verifyPositionExists checks if a position still exists in MT5
func (p *Processor) verifyPositionExists(ctx context.Context, positionTicket int64) bool {
	if !p.mt5Client.IsConnected(ctx) {
		return false
	}
	
	positions, err := p.mt5Client.GetPositions(ctx)
	if err != nil {
		log.Printf("Failed to get positions for verification: %v", err)
		return false
	}
	
	for _, pos := range positions {
		if pos.Ticket == positionTicket {
			return true
		}
	}
	
	return false
}
