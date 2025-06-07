#!/bin/bash

# Test MT5 Integration Script
SERVER_URL="http://localhost:8081"
MT5_BRIDGE_URL="http://localhost:8080"

echo "🧪 Testing MT5 Trading System Integration..."
echo "=============================================="

# Test trading engine health
echo "1. Testing Trading Engine health..."
curl -s "$SERVER_URL/health" | jq .
echo ""

# Test MT5 bridge (if available)
echo "2. Testing MT5 Bridge connection..."
if curl -s --connect-timeout 3 "$MT5_BRIDGE_URL/health" > /dev/null 2>&1; then
    echo "✅ MT5 Bridge is running"
    curl -s "$MT5_BRIDGE_URL/health" | jq .
else
    echo "⚠️ MT5 Bridge not available at $MT5_BRIDGE_URL"
    echo "   Start it with: ./mt5-bridge/run_bridge.sh"
fi
echo ""

# Test MT5 status endpoint
echo "3. Testing MT5 status endpoint..."
curl -s "$SERVER_URL/mt5/status" | jq .
echo ""

# Test trades endpoint
echo "4. Testing trades endpoint..."
curl -s "$SERVER_URL/trades" | jq .
echo ""

# Test positions endpoint
echo "5. Testing positions endpoint..."
curl -s "$SERVER_URL/positions" | jq .
echo ""

# Test TradingView webhook with MT5 integration
echo "6. Testing TradingView webhook with buy signal..."
curl -s -X POST "$SERVER_URL/webhook/tradingview" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EURUSD",
    "action": "buy",
    "price": 1.0850,
    "stop_loss": 1.0800,
    "take_profit": 1.0900,
    "volume": 0.01,
    "message": "MT5 Integration test buy signal",
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }' | jq .
echo ""

# Wait a moment for processing
echo "⏳ Waiting 3 seconds for signal processing..."
sleep 3

# Check trades again to see if the signal was processed
echo "7. Checking trades after signal processing..."
curl -s "$SERVER_URL/trades" | jq .
echo ""

# Test close signal
echo "8. Testing close signal..."
curl -s -X POST "$SERVER_URL/webhook/tradingview" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EURUSD",
    "action": "close",
    "message": "MT5 Integration test close signal",
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }' | jq .
echo ""

echo "=============================================="
echo "✅ MT5 Integration tests completed!"
echo ""
echo "📊 For continuous monitoring, check these endpoints:"
echo "   - Trading Engine: $SERVER_URL/health"
echo "   - MT5 Status: $SERVER_URL/mt5/status"
echo "   - Active Trades: $SERVER_URL/trades"
echo "   - Live Positions: $SERVER_URL/positions"
echo ""
echo "💡 To fully test MT5 execution:"
echo "   1. Install MetaTrader 5 on Windows VPS"
echo "   2. Start MT5 bridge: ./mt5-bridge/run_bridge.sh"
echo "   3. Configure MT5 to allow DLL imports"
echo "   4. Run this test script again" 