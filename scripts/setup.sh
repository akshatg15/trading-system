#!/bin/bash

# Trading System Setup Script
# Comprehensive setup for the complete trading system

set -e  # Exit on any error

echo "🚀 Trading System Setup"
echo "======================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# Check if Go is installed
check_go() {
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}')
        print_status "Go is installed: $GO_VERSION"
        return 0
    else
        print_error "Go is not installed"
        print_info "Please install Go from: https://golang.org/dl/"
        return 1
    fi
}

# Check if Python is installed  
check_python() {
    if command -v python3 &> /dev/null; then
        PYTHON_VERSION=$(python3 --version)
        print_status "Python is installed: $PYTHON_VERSION"
        return 0
    else
        print_error "Python 3 is not installed"
        print_info "Please install Python 3.9+ from: https://python.org/downloads/"
        return 1
    fi
}

# Setup Go dependencies
setup_go() {
    print_info "Setting up Go dependencies..."
    
    # Initialize Go module if not exists
    if [ ! -f "go.mod" ]; then
        print_info "Initializing Go module..."
        go mod init trading-system
    fi
    
    # Download dependencies
    print_info "Downloading Go dependencies..."
    go mod tidy
    
    # Build the application
    print_info "Building Go application..."
    go build -o bin/trading-engine ./cmd/trading-engine
    
    print_status "Go setup complete"
}

# Setup MT5 Bridge with UV
setup_mt5_bridge() {
    print_info "Setting up MT5 Bridge with UV (modern Python package manager)..."
    
    # Check if setup script exists
    if [ -f "scripts/setup_mt5_bridge_uv.py" ]; then
        python3 scripts/setup_mt5_bridge_uv.py
    else
        print_warning "UV setup script not found, falling back to traditional setup..."
        
        # Fallback to traditional setup
        if [ -f "scripts/setup_mt5_bridge.py" ]; then
            python3 scripts/setup_mt5_bridge.py
        else
            print_error "No MT5 bridge setup script found"
            return 1
        fi
    fi
    
    print_status "MT5 Bridge setup complete"
}

# Create necessary directories
create_directories() {
    print_info "Creating necessary directories..."
    
    mkdir -p bin
    mkdir -p logs
    mkdir -p configs
    mkdir -p data
    
    print_status "Directories created"
}

# Setup configuration files
setup_config() {
    print_info "Setting up configuration files..."
    
    # Create .env file if it doesn't exist
    if [ ! -f ".env" ]; then
        print_info "Creating .env file..."
        cat > .env << EOF
# Trading System Configuration

# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8081
WEBHOOK_SECRET=your-webhook-secret-change-this

# Database Configuration  
DATABASE_URL=sqlite:///data/trading.db

# MT5 Bridge Configuration
MT5_BRIDGE_URL=http://localhost:8080

# Risk Management
RISK_MAX_DAILY_LOSS=100.00
RISK_MAX_POSITION_SIZE=0.01
RISK_MAX_OPEN_POSITIONS=5
RISK_ENABLED=true

# Logging
LOG_LEVEL=info
LOG_FILE=logs/trading-system.log

# Development Mode
DEBUG=false
DEVELOPMENT_MODE=true
EOF
        print_status ".env file created"
    else
        print_warning ".env file already exists, skipping..."
    fi
    
    # Create sample config.yaml if it doesn't exist
    if [ ! -f "configs/config.yaml" ]; then
        print_info "Creating sample config.yaml..."
        mkdir -p configs
        cat > configs/config.yaml << EOF
# Trading System Configuration

server:
  host: "0.0.0.0"
  port: 8081
  webhook_secret: "your-webhook-secret-change-this"

database:
  url: "sqlite:///data/trading.db"
  
mt5:
  bridge_url: "http://localhost:8080"
  
risk:
  max_daily_loss: 100.00
  max_position_size: 0.01
  max_open_positions: 5
  enabled: true

logging:
  level: "info"
  file: "logs/trading-system.log"
EOF
        print_status "Sample config.yaml created"
    else
        print_warning "config.yaml already exists, skipping..."
    fi
}

# Setup database
setup_database() {
    print_info "Setting up database..."
    
    # Create migrations directory if it doesn't exist
    mkdir -p migrations
    
    # Run database migrations if binary exists
    if [ -f "bin/trading-engine" ]; then
        print_info "Running database migrations..."
        ./bin/trading-engine -migrate || print_warning "Database migration failed or not implemented"
    else
        print_warning "Trading engine binary not found, skipping database setup"
    fi
    
    print_status "Database setup complete"
}

# Test the installation
test_installation() {
    print_info "Testing installation..."
    
    # Test Go build
    if [ -f "bin/trading-engine" ]; then
        print_status "Go binary built successfully"
    else
        print_error "Go binary not found"
        return 1
    fi
    
    # Test MT5 Bridge (if available)
    if command -v uv &> /dev/null; then
        print_info "Testing MT5 Bridge with UV..."
        cd mt5-bridge
        if uv run python -c "import flask, MetaTrader5, requests; print('✅ MT5 Bridge imports OK')"; then
            print_status "MT5 Bridge test passed"
        else
            print_warning "MT5 Bridge test failed (may need MT5 terminal)"
        fi
        cd ..
    else
        print_warning "UV not available, skipping MT5 Bridge test"
    fi
    
    print_status "Installation test complete"
}

# Print usage instructions
print_usage() {
    echo ""
    echo "=================================================="
    echo "🎉 Trading System Setup Complete!"
    echo "=================================================="
    echo ""
    echo "📋 Quick Start:"
    echo ""
    echo "1. Start the Go trading engine:"
    echo "   ./bin/trading-engine"
    echo ""
    echo "2. Start the MT5 Bridge (in another terminal):"
    echo "   cd mt5-bridge"
    if command -v uv &> /dev/null; then
        echo "   uv run python mt5_bridge.py"
    else
        echo "   python3 mt5_bridge.py"
    fi
    echo ""
    echo "3. Test the system:"
    echo "   curl http://localhost:8081/health"
    echo "   curl http://localhost:8080/health"
    echo ""
    echo "📖 Next Steps:"
    echo ""
    echo "1. Configure your .env file with proper settings"
    echo "2. Set up your MT5 terminal and broker account"
    echo "3. Configure TradingView webhooks"
    echo "4. Read the documentation in docs/"
    echo ""
    echo "🔗 Architecture Options:"
    echo ""
    echo "• Simple: TradingView → Go Engine (port 8081) → MT5 Bridge → MT5"
    echo "• Enterprise: TradingView → Cloudflare Worker → Go Engine → MT5"
    echo "• Alternative: TradingView → Expert Advisor (port 8080) → MT5"
    echo ""
    echo "📚 Documentation:"
    echo "• Architecture Decision: docs/ARCHITECTURE_DECISION.md"
    echo "• Broker Setup: docs/BROKER_SETUP.md" 
    echo "• MT5 EA Alternative: mt5-ea/README.md"
    echo ""
    echo "⚠️  Important:"
    echo "• Update WEBHOOK_SECRET in .env before production use"
    echo "• Configure proper risk management settings"
    echo "• Test with demo account first"
    echo ""
    print_warning "For Windows users: Install and configure MT5 terminal"
    print_warning "For development: The system works without MT5 (simulated mode)"
}

# Main setup function
main() {
    echo "Starting comprehensive setup..."
    echo ""
    
    # Check prerequisites
    print_info "Checking prerequisites..."
    if ! check_go || ! check_python; then
        print_error "Prerequisites not met. Please install required software."
        exit 1
    fi
    
    # Create directories
    create_directories
    
    # Setup Go application
    setup_go
    
    # Setup MT5 Bridge
    setup_mt5_bridge
    
    # Setup configuration
    setup_config
    
    # Setup database
    setup_database
    
    # Test installation
    test_installation
    
    # Print usage instructions
    print_usage
    
    print_status "Setup completed successfully! 🎉"
}

# Run main function
main "$@" 