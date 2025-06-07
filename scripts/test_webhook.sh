#!/bin/bash

# Test TradingView Webhook Script
SERVER_URL="http://localhost:8081"

echo "Testing MT5 Trading System Webhooks..."

# Test health endpoint
echo "1. Testing health endpoint..."
curl -s "$SERVER_URL/health" | jq .
echo ""

# Test TradingView webhook with buy signal
echo "2. Testing buy signal webhook..."
curl -s -X POST "$SERVER_URL/webhook/tradingview" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EURUSD",
    "action": "buy",
    "price": 1.0850,
    "stop_loss": 1.0800,
    "take_profit": 1.0900,
    "volume": 0.01,
    "message": "Test buy signal",
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }' | jq .
echo ""

# Test TradingView webhook with sell signal
echo "3. Testing sell signal webhook..."
curl -s -X POST "$SERVER_URL/webhook/tradingview" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "GBPUSD",
    "action": "sell",
    "price": 1.2650,
    "stop_loss": 1.2700,
    "take_profit": 1.2600,
    "volume": 0.01,
    "message": "Test sell signal",
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }' | jq .
echo ""

# Test TradingView webhook with close signal
echo "4. Testing close signal webhook..."
curl -s -X POST "$SERVER_URL/webhook/tradingview" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EURUSD",
    "action": "close",
    "message": "Test close signal",
    "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }' | jq .
echo ""

echo "Webhook tests completed!"
echo "Check the trading engine logs for signal processing status." 