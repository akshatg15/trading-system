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
	volume := p.calculatePositionSize(signal.Symbol, requestedVolume)

	// Create entry trade
	entryTrade, err := p.createEntryTrade(ctx, signal, volume)
	if err != nil {
		return fmt.Errorf("failed to create entry trade: %w", err)
	}

	log.Printf("Created entry trade %d for signal %d: %s %s %.2f lots",
		entryTrade.ID, signal.ID, entryTrade.Direction, entryTrade.Symbol, entryTrade.Volume)

	// Execute entry trade via MT5
	if err := p.executeTrade(ctx, entryTrade); err != nil {
		log.Printf("Failed to execute entry trade %d: %v", entryTrade.ID, err)
		// Update trade status but don't return error - signal should still be marked as processed
		// since the trade record was created successfully
		p.updateTradeStatus(ctx, entryTrade.ID, "rejected", nil)
		log.Printf("Trade %d marked as rejected due to execution failure", entryTrade.ID)
		// Continue with TP trades creation even if entry failed (they'll be in pending state)
	}

	// Create TP1 and TP2 trades if available
	if signal.TP1 != nil && *signal.TP1 > 0 {
		tp1Trade, err := p.createTPTrade(ctx, signal, "tp1", *signal.TP1, volume/2) // 50% for TP1
		if err != nil {
			log.Printf("Failed to create TP1 trade: %v", err)
		} else {
			log.Printf("Created TP1 trade %d for signal %d", tp1Trade.ID, signal.ID)
		}
	}

	if signal.TP2 != nil && *signal.TP2 > 0 {
		tp2Trade, err := p.createTPTrade(ctx, signal, "tp2", *signal.TP2, volume/2) // 50% for TP2
		if err != nil {
			log.Printf("Failed to create TP2 trade: %v", err)
		} else {
			log.Printf("Created TP2 trade %d for signal %d", tp2Trade.ID, signal.ID)
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
func (p *Processor) createTPTrade(ctx context.Context, signal *database.Signal, tpType string, tpPrice float64, volume float64) (*database.Trade, error) {
	tradeReq := &database.CreateTradeRequest{
		SignalID:       &signal.ID,
		ParentSignalID: &signal.ID,
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

// executeTrade sends the trade to MT5 for execution
func (p *Processor) executeTrade(ctx context.Context, trade *database.Trade) error {
	// Check if MT5 is connected
	if !p.mt5Client.IsConnected(ctx) {
		return fmt.Errorf("MT5 bridge not available")
	}

	// Prepare MT5 trade request
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
	if trade.TakeProfit != nil {
		mt5Req.TakeProfit = *trade.TakeProfit
	}

	// Send trade to MT5
	response, err := p.mt5Client.SendTrade(ctx, mt5Req)
	if err != nil {
		return fmt.Errorf("failed to send trade to MT5: %w", err)
	}

	// Update trade with MT5 response
	responseData, _ := json.Marshal(response)
	responseRaw := json.RawMessage(responseData)

	updateReq := &database.UpdateTradeStatusRequest{
		MT5Response: &responseRaw,
	}

	if response.Success {
		updateReq.Status = "filled"
		updateReq.MT5Ticket = &response.Ticket
		updateReq.EntryPrice = &response.Price
		if response.Commission != 0 {
			updateReq.Commission = &response.Commission
		}
	} else {
		updateReq.Status = "rejected"
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

	// Get positions from MT5
	positions, err := p.mt5Client.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get positions from MT5: %w", err)
	}

	// Create map of MT5 tickets for quick lookup
	mt5Tickets := make(map[int64]*mt5.PositionInfo)
	for _, pos := range positions {
		mt5Tickets[pos.Ticket] = pos
	}

	// Update trades with current MT5 data
	for _, trade := range openTrades {
		if trade.MT5Ticket == nil {
			continue
		}

		if pos, exists := mt5Tickets[*trade.MT5Ticket]; exists {
			// Position still exists in MT5, update current data
			updateReq := &database.UpdateTradeStatusRequest{
				CurrentPrice: &pos.CurrentPrice,
				ProfitLoss:   &pos.Profit,
				Commission:   &pos.Commission,
				Swap:         &pos.Swap,
			}

			if err := p.db.UpdateTradeStatus(ctx, trade.ID, updateReq); err != nil {
				log.Printf("Failed to update trade %d: %v", trade.ID, err)
			}
		} else {
			// Position no longer exists in MT5, mark as closed
			log.Printf("Trade %d (MT5 ticket %d) no longer exists in MT5, marking as closed", trade.ID, *trade.MT5Ticket)
			p.updateTradeStatus(ctx, trade.ID, "closed", nil)
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
	return 0.01
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
