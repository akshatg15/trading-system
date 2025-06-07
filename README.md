# MT5 Trading Auto-System

An automated trading system that receives signals from TradingView and executes trades via MetaTrader 5.

## 🏗️ Current Status: Phase 2 Complete ✅

**Implemented Components:**
- ✅ **Go Trading Engine**: Complete signal processing and database operations
- ✅ **PostgreSQL Schema**: Database tables for signals, trades, logs, and risk events  
- ✅ **TradingView Webhook Handler**: Secure webhook processing with signature validation
- ✅ **Risk Management**: Position sizing and risk validation framework
- ✅ **Configuration Management**: Environment-based configuration with validation
- ✅ **Signal Processing Loop**: Automated processing every 2 seconds
- ✅ **MT5 HTTP Client**: Go client for communicating with MT5 bridge
- ✅ **MT5 Python Bridge**: HTTP bridge server for MetaTrader 5 integration
- ✅ **Trade Execution**: Full trade lifecycle from signal to MT5 execution
- ✅ **Position Monitoring**: Real-time position sync between database and MT5
- ✅ **Monitoring Endpoints**: API endpoints for trades, positions, and MT5 status

**Pending Components:**
- 🟡 **Cloudflare Worker**: Webhook relay service (Phase 3)
- 🟡 **Production Deployment**: VPS setup and monitoring (Phase 3)

## Architecture

```
TradingView → Go Engine → PostgreSQL
                ↓
        MT5 Python Bridge → MT5 Terminal
```

## Components

- `cmd/trading-engine/` - Go trading engine main application
- `internal/` - Core business logic modules (config, database, signals, server, mt5)
- `mt5-bridge/` - Python HTTP bridge for MetaTrader 5 communication
- `scripts/` - Setup, deployment and testing scripts
- `bin/` - Built binaries
- `cf-worker/` - Cloudflare Worker for webhook handling (Phase 3)

## Quick Start

### 1. Setup

```bash
# Run complete setup (includes MT5 bridge)
./scripts/setup.sh

# Edit configuration (required)
cp config.example .env
# Update .env with your database credentials and webhook secret
```

### 2. Database Setup

```bash
# Initialize Neon PostgreSQL database
psql $DATABASE_URL -f scripts/init_db.sql
```

### 3. Start MT5 Bridge (Windows Required)

```bash
# On Windows VPS with MT5 installed:
./mt5-bridge/run_bridge.bat

# On other platforms (development mode):
./mt5-bridge/run_bridge.sh
```

### 4. Start Trading Engine

```bash
# Start the main trading engine
./bin/trading-engine

# Or run from source
cd cmd/trading-engine
go run main.go
```

### 5. Test Integration

```bash
# Test complete system integration
./scripts/test_mt5_integration.sh

# Test just webhooks
./scripts/test_webhook.sh
```

## Configuration

Key environment variables in `.env`:

```bash
# Database (Neon PostgreSQL)
DATABASE_URL=postgresql://neondb_owner:password@host.neon.tech/neondb?sslmode=require

# Security
WEBHOOK_SECRET=your_secure_random_string

# MT5 Integration
MT5_ENDPOINT=http://localhost:8080
MT5_TIMEOUT_SECONDS=10
MT5_RETRY_ATTEMPTS=3

# Risk Management
RISK_MAX_DAILY_LOSS=1000.00
RISK_MAX_POSITION_SIZE=0.1
RISK_MAX_OPEN_POSITIONS=3

# Server
SERVER_PORT=8081
```

## API Endpoints

### Trading Engine (Port 8081)
- `GET /health` - Health check
- `POST /webhook/tradingview` - TradingView webhook receiver
- `GET /trades` - Get all trades
- `GET /positions` - Get current MT5 positions
- `GET /mt5/status` - MT5 connection and account status

### MT5 Bridge (Port 8080)
- `GET /health` - MT5 bridge health check
- `POST /trade` - Execute trade in MT5
- `GET /positions` - Get MT5 positions
- `GET /account` - Get MT5 account info

## TradingView Webhook Format

```json
{
  "ticker": "EURUSD",
  "action": "buy",
  "price": 1.0850,
  "stop_loss": 1.0800, 
  "take_profit": 1.0900,
  "volume": 0.01,
  "message": "Signal description",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## MT5 Setup Requirements

### Windows VPS Setup:
1. Install MetaTrader 5
2. Enable "Allow DLL imports" in Tools > Options > Expert Advisors
3. Ensure Python 3.8+ is installed
4. Run MT5 bridge setup: `python scripts/setup_mt5_bridge.py`

### Development Setup (Non-Windows):
- MT5 bridge will run in development mode
- Trade execution will be simulated
- All monitoring endpoints remain functional

## Trade Flow

1. **Signal Reception**: TradingView sends webhook to `/webhook/tradingview`
2. **Signal Processing**: Go engine validates and processes signal every 2 seconds
3. **Risk Management**: Position sizing and risk checks applied
4. **Trade Creation**: Trade record created in PostgreSQL database
5. **MT5 Execution**: Trade sent to MT5 via Python bridge
6. **Position Monitoring**: Real-time sync of position data every 10 seconds
7. **Trade Updates**: Status, P&L, and commission updated automatically

## Next Phase

### Phase 3: Production Deployment  
- Cloudflare Worker for global webhook handling
- VPS deployment automation
- Enhanced monitoring and alerting
- Performance optimization

## Development

```bash
# Install dependencies
go mod tidy

# Build
go build ./cmd/trading-engine

# Run tests
go test ./...

# Format code  
go fmt ./...

# Setup MT5 bridge
python3 scripts/setup_mt5_bridge.py
```

## Monitoring

The system provides comprehensive logging and monitoring:

- **Database Logs**: All events stored in `system_logs` table
- **Console Output**: Structured JSON logging
- **HTTP Endpoints**: Real-time status via REST API
- **Position Sync**: Automatic reconciliation between database and MT5

Monitor system health and performance through the provided endpoints and log files. 