# MT5 Trading Bridge

A Python-based bridge for interacting with MetaTrader 5 (MT5) trading platform. This bridge provides a REST API interface to MT5, allowing for automated trading and integration with other systems.

## Features

- REST API interface to MT5
- Real-time market data streaming
- Order management
- Account information
- Risk management controls
- Webhook support for notifications

## Installation

1. Ensure you have Python 3.9+ installed
2. Install uv package manager
3. Run the setup script:
   ```bash
   python scripts/setup_mt5_bridge_uv.py
   ```

## Configuration

1. Copy `.env.example` to `.env`
2. Configure your MT5 connection settings
3. Set up your risk management parameters
4. Configure webhook settings if needed

## Usage

Start the bridge:
```bash
uv run python mt5_bridge.py
```

The API will be available at `http://localhost:8080`

## API Endpoints

- `GET /health` - Check bridge status
- `GET /account` - Get account information
- `POST /order` - Place new order
- `GET /positions` - Get open positions
- `GET /market-data` - Get market data

## Development

- `uv add package-name` - Add new dependency
- `uv remove package-name` - Remove dependency
- `uv run pytest` - Run tests
- `uv run black .` - Format code
- `uv run ruff check .` - Lint code

## License

MIT License 