# MT5 Expert Advisor - Alternative Implementation

This directory contains **Expert Advisor (EA) files** for MetaTrader 5 that provide an **alternative to the Python bridge** approach.

## 🎯 Two Implementation Options

### Option 1: Python Bridge (Current Implementation)
```
TradingView → Go Engine → Python Bridge → MT5 Terminal
```

### Option 2: Expert Advisor (This Directory)  
```
TradingView → Expert Advisor → MT5 Terminal
```

## 🔧 Expert Advisor Features

The **TradingSystemEA.mq5** provides:

- ✅ **HTTP Server**: Built-in web server on port 8080
- ✅ **Webhook Processing**: Direct TradingView webhook handling
- ✅ **Risk Management**: Daily loss limits, position limits
- ✅ **Symbol Filtering**: Configurable allowed symbols
- ✅ **Trade Execution**: Buy/Sell/Close operations
- ✅ **Position Monitoring**: Real-time position tracking
- ✅ **Account Info**: Balance, equity, margin information
- ✅ **JSON API**: RESTful endpoints for monitoring

## 📋 Installation Guide

### Step 1: Copy EA to MT5
```bash
# Windows MT5 Data Folder:
C:\Users\[Username]\AppData\Roaming\MetaQuotes\Terminal\[TerminalID]\MQL5\Experts\

# Copy TradingSystemEA.mq5 to the Experts folder
```

### Step 2: Compile EA
1. Open **MetaEditor** (F4 in MT5)
2. Open `TradingSystemEA.mq5`
3. Click **Compile** (F7)
4. Check for any errors in the log

### Step 3: Attach to Chart
1. In MT5, open any chart (symbol doesn't matter)
2. Go to **Navigator → Expert Advisors**
3. Drag `TradingSystemEA` onto the chart
4. Configure parameters (see below)
5. Click **OK**

### Step 4: Enable Auto Trading
```
Tools → Options → Expert Advisors
✅ Allow automated trading
✅ Allow DLL imports
✅ Allow imports of external experts
```

## ⚙️ Configuration Parameters

### Basic Settings
```mql5
WebhookSecret = "your-webhook-secret"    // Must match TradingView
ServerPort = 8080                        // HTTP server port
DefaultLotSize = 0.01                    // Default position size
MagicNumber = 123456                     // Unique EA identifier
```

### Risk Management
```mql5
MaxDailyLoss = 100.0                     // Max daily loss ($100)
MaxOpenPositions = 5                     // Max concurrent positions
EnableRiskManagement = true              // Enable/disable risk limits
AllowedSymbols = "EURUSD,GBPUSD,XAUUSD" // Comma-separated symbols
```

## 🌐 API Endpoints

Once the EA is running, it provides these endpoints:

### Health Check
```bash
GET http://your-vps:8080/health

Response:
{
  "status": "healthy",
  "timestamp": "2024-01-15 10:30:00",
  "account": 12345,
  "balance": 10000.0,
  "equity": 10150.0,
  "daily_pnl": 150.0
}
```

### TradingView Webhook
```bash
POST http://your-vps:8080/webhook/tradingview

Body:
{
  "ticker": "EURUSD", 
  "action": "BUY",
  "qty": 0.01,
  "price": 1.0850
}
```

### Get Positions
```bash
GET http://your-vps:8080/positions

Response:
{
  "positions": [
    {
      "ticket": 123456,
      "symbol": "EURUSD",
      "type": "POSITION_TYPE_BUY",
      "volume": 0.01,
      "price_open": 1.0850,
      "price_current": 1.0855,
      "profit": 0.50,
      "time": "2024-01-15 10:25:00"
    }
  ]
}
```

### Get Account Info
```bash
GET http://your-vps:8080/account

Response:
{
  "login": 12345,
  "balance": 10000.0,
  "equity": 10150.0,
  "margin": 21.70,
  "free_margin": 9978.30,
  "margin_level": 467.74,
  "currency": "USD",
  "company": "Exness",
  "server": "Exness-MT5Real",
  "daily_pnl": 150.0
}
```

## 🔧 TradingView Configuration

### Webhook URL
```
http://your-vps-ip:8080/webhook/tradingview
```

### Webhook Message Format
```json
{
  "ticker": "{{ticker}}",
  "action": "{{strategy.order.action}}", 
  "qty": "{{strategy.order.contracts}}",
  "price": "{{close}}"
}
```

### Supported Actions
- `BUY` or `buy` - Open long position
- `SELL` or `sell` - Open short position  
- `CLOSE` or `close` - Close positions for symbol
- `CLOSE_ALL` or `close_all` - Close all positions

## 🆚 Comparison: EA vs Python Bridge

| Feature | Expert Advisor | Python Bridge |
|---------|---------------|---------------|
| **Setup Complexity** | Medium (MT5 only) | High (Go + Python + MT5) |
| **Performance** | High (native MT5) | Medium (network calls) |
| **Reliability** | High (single process) | Medium (multiple processes) |
| **Debugging** | MT5 logs only | Full logging stack |
| **Customization** | MQL5 required | Python/Go flexibility |
| **Resource Usage** | Low | Medium |
| **Maintenance** | Low | Medium |

## 🎯 When to Use Expert Advisor

**✅ Choose EA if:**
- You want **simplicity** and fewer moving parts
- You're comfortable with **MQL5 programming**
- You need **maximum performance** 
- You want **native MT5 integration**
- You prefer **single-component** solution

**❌ Avoid EA if:**
- You need **complex logic** beyond trading
- You want **external database** integration
- You need **advanced logging/monitoring**
- You're not familiar with **MQL5**

## 🚀 Quick Start (EA Path)

```bash
# 1. Skip Python bridge setup entirely
# 2. Copy TradingSystemEA.mq5 to MT5 Experts folder
# 3. Compile and attach to chart
# 4. Configure TradingView webhook:
#    URL: http://your-vps:8080/webhook/tradingview
# 5. Start trading!
```

## 🐛 Troubleshooting

### EA Not Starting
- Check **Expert Advisors** tab for errors
- Ensure **Auto Trading** is enabled
- Verify **port 8080** is not in use
- Check Windows firewall settings

### Webhook Not Working
- Test with: `curl -X POST http://your-vps:8080/health`
- Verify **WebhookSecret** matches TradingView
- Check MT5 **Expert** tab for logs
- Ensure **AllowedSymbols** includes your instruments

### Trades Not Executing
- Check **account balance** and **margin**
- Verify **symbol availability** in MT5
- Check **daily loss limits**
- Review **MagicNumber** conflicts

## 📝 Development Notes

### Adding Custom Logic
Edit `TradingSystemEA.mq5` to add:
- **Stop loss/Take profit** calculations
- **Advanced risk management** rules
- **Custom position sizing** algorithms
- **Technical indicator** integration
- **Multi-timeframe** analysis

### JSON Library
The EA uses `JSON.mqh` library. If not available:
1. Download from MQL5 community
2. Place in `Include` folder
3. Recompile EA

This provides a **complete alternative** to the Python bridge for users who prefer pure MT5 implementation! 