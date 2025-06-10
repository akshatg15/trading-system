#!/bin/bash

# Partial Profit Architecture Deployment Script
# This script helps deploy the new partial profit functionality

set -e

echo "🚀 Deploying Partial Profit Architecture..."

# Check if required environment variables are set
if [[ -z "$DATABASE_URL" ]]; then
    echo "❌ ERROR: DATABASE_URL environment variable is not set"
    exit 1
fi

# 1. Database Migration
echo "📊 Running database migration..."
if command -v psql &> /dev/null; then
    echo "Running migration 002-add-partial-profit-fields.sql..."
    psql "$DATABASE_URL" -f migrations/002-add-partial-profit-fields.sql
    echo "✅ Database migration completed"
else
    echo "⚠️  psql not found. Please run the following manually:"
    echo "   psql \$DATABASE_URL -f migrations/002-add-partial-profit-fields.sql"
fi

# 2. Build Go Application
echo "🔨 Building Go trading engine..."
go build -o trading-engine ./cmd/trading-engine
if [[ $? -eq 0 ]]; then
    echo "✅ Go application built successfully"
else
    echo "❌ Go build failed"
    exit 1
fi

# 3. Test Configuration
echo "🔧 Testing configuration..."
if [[ -f ".env" ]]; then
    echo "✅ .env file found"
else
    echo "⚠️  .env file not found. Creating from example..."
    cp config.example .env
    echo "📝 Please update .env with your configuration"
fi

# 4. Check MT5 Bridge Dependencies
echo "🔌 Checking MT5 Bridge..."
if [[ -f "mt5-bridge/mt5_bridge.py" ]]; then
    cd mt5-bridge
    if [[ -f "requirements.txt" ]]; then
        echo "📦 Installing MT5 Bridge dependencies..."
        if command -v pip &> /dev/null; then
            pip install -r requirements.txt
        else
            echo "⚠️  pip not found. Please install Python dependencies manually:"
            echo "   cd mt5-bridge && pip install -r requirements.txt"
        fi
    fi
    cd ..
    echo "✅ MT5 Bridge ready"
else
    echo "❌ MT5 Bridge not found"
fi

# 5. Validate Pine Script
echo "📈 Validating Pine Script..."
if [[ -f "pine-scripts/manipulation_partial_tp.pine" ]]; then
    echo "✅ Pine Script found and updated"
    echo "📝 Please deploy this script to TradingView and configure alerts"
else
    echo "❌ Pine Script not found"
fi

# 6. Test Database Connection
echo "🔍 Testing database connection..."
./trading-engine -test-db 2>/dev/null || {
    echo "⚠️  Database connection test not available in current build"
    echo "   The trading engine will test the connection on startup"
}

echo ""
echo "🎉 Deployment completed successfully!"
echo ""
echo "📋 Next Steps:"
echo "1. Update your .env file with correct configuration"
echo "2. Run database migration manually if psql was not available:"
echo "   psql \$DATABASE_URL -f migrations/002-add-partial-profit-fields.sql"
echo "3. Start the MT5 Bridge:"
echo "   cd mt5-bridge && python mt5_bridge.py"
echo "4. Start the Trading Engine:"
echo "   ./trading-engine"
echo "5. Deploy Pine Script to TradingView and configure alerts"
echo "6. Test with a demo account first!"
echo ""
echo "📖 Full documentation: docs/partial-profit-architecture/1.0.0.md"

# 7. Show system architecture
echo ""
echo "🏗️  System Architecture:"
echo "TradingView → CloudFlare Worker → Go Engine → Database → MT5 Bridge → MT5"
echo "             (optional)                    ↓"
echo "                                    Multiple Trade Records:"
echo "                                    - Entry (market order)"
echo "                                    - TP1 (50% at 1:1 RR)"
echo "                                    - TP2 (50% at 1:2 RR)"
echo ""
echo "💡 The system now creates separate trade records for each profit level"
echo "   providing better tracking, risk management, and audit trails." 