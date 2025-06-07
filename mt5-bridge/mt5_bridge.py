#!/usr/bin/env python3
"""
MT5 HTTP Bridge Server
Provides REST API interface to MetaTrader 5 for the trading system.
"""

import json
import logging
import time
from datetime import datetime
from typing import Dict, List, Optional, Any

import MetaTrader5 as mt5
from flask import Flask, request, jsonify
from flask_cors import CORS

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)
CORS(app)

class MT5Bridge:
    def __init__(self):
        self.connected = False
        self.initialize_mt5()
    
    def initialize_mt5(self) -> bool:
        """Initialize MetaTrader 5 connection."""
        try:
            if not mt5.initialize():
                error = mt5.last_error()
                logger.error(f"Failed to initialize MT5: {error}")
                return False
            
            self.connected = True
            logger.info("✅ MT5 initialized successfully")
            
            # Get account info
            account_info = mt5.account_info()
            if account_info:
                logger.info(f"Connected to account: {account_info.login}")
                logger.info(f"Balance: {account_info.balance} {account_info.currency}")
            
            return True
            
        except Exception as e:
            logger.error(f"Error initializing MT5: {e}")
            return False
    
    def is_connected(self) -> bool:
        """Check if MT5 is connected."""
        if not self.connected:
            return False
        
        try:
            account_info = mt5.account_info()
            return account_info is not None
        except:
            return False
    
    def execute_trade(self, trade_data: Dict[str, Any]) -> Dict[str, Any]:
        """Execute a trade in MT5."""
        try:
            symbol = trade_data.get('symbol')
            action = trade_data.get('action')  # 'buy', 'sell', 'close'
            volume = float(trade_data.get('volume', 0.01))
            order_type = trade_data.get('order_type', 'market')
            price = trade_data.get('price', 0.0)
            stop_loss = trade_data.get('stop_loss', 0.0)
            take_profit = trade_data.get('take_profit', 0.0)
            comment = trade_data.get('comment', 'AutoTrader')
            magic = trade_data.get('magic', 123456)
            
            # Validate symbol
            if not mt5.symbol_select(symbol, True):
                return {
                    'success': False,
                    'error_code': 4106,
                    'error_msg': f'Symbol {symbol} not found'
                }
            
            # Get symbol info
            symbol_info = mt5.symbol_info(symbol)
            if not symbol_info:
                return {
                    'success': False,
                    'error_code': 4106,
                    'error_msg': f'Failed to get symbol info for {symbol}'
                }
            
            # Handle close action
            if action == 'close':
                return self._close_position(trade_data)
            
            # Determine order type
            if action == 'buy':
                order_type_mt5 = mt5.ORDER_TYPE_BUY if order_type == 'market' else mt5.ORDER_TYPE_BUY_LIMIT
                if order_type == 'market':
                    price = mt5.symbol_info_tick(symbol).ask
            elif action == 'sell':
                order_type_mt5 = mt5.ORDER_TYPE_SELL if order_type == 'market' else mt5.ORDER_TYPE_SELL_LIMIT
                if order_type == 'market':
                    price = mt5.symbol_info_tick(symbol).bid
            else:
                return {
                    'success': False,
                    'error_code': 4000,
                    'error_msg': f'Invalid action: {action}'
                }
            
            # Prepare trade request
            request_dict = {
                "action": mt5.TRADE_ACTION_DEAL,
                "symbol": symbol,
                "volume": volume,
                "type": order_type_mt5,
                "price": price,
                "magic": magic,
                "comment": comment,
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": mt5.ORDER_FILLING_IOC,
            }
            
            # Add stop loss and take profit if specified
            if stop_loss > 0:
                request_dict["sl"] = stop_loss
            if take_profit > 0:
                request_dict["tp"] = take_profit
            
            # Send order
            result = mt5.order_send(request_dict)
            
            if result.retcode != mt5.TRADE_RETCODE_DONE:
                return {
                    'success': False,
                    'error_code': result.retcode,
                    'error_msg': f'Order failed: {result.comment}'
                }
            
            return {
                'success': True,
                'ticket': result.order,
                'volume': result.volume,
                'price': result.price,
                'commission': 0.0,  # Will be updated later
                'swap': 0.0,
                'profit': 0.0
            }
            
        except Exception as e:
            logger.error(f"Error executing trade: {e}")
            return {
                'success': False,
                'error_code': 5000,
                'error_msg': str(e)
            }
    
    def _close_position(self, trade_data: Dict[str, Any]) -> Dict[str, Any]:
        """Close a specific position."""
        try:
            magic = trade_data.get('magic')
            
            if not magic:
                return {
                    'success': False,
                    'error_code': 4000,
                    'error_msg': 'Magic number required for close operation'
                }
            
            # Find position by ticket (magic number used as ticket)
            positions = mt5.positions_get(ticket=magic)
            
            if not positions:
                return {
                    'success': False,
                    'error_code': 4051,
                    'error_msg': f'Position {magic} not found'
                }
            
            position = positions[0]
            
            # Determine close order type
            close_type = mt5.ORDER_TYPE_SELL if position.type == mt5.POSITION_TYPE_BUY else mt5.ORDER_TYPE_BUY
            
            # Get current price
            symbol_info = mt5.symbol_info_tick(position.symbol)
            close_price = symbol_info.bid if position.type == mt5.POSITION_TYPE_BUY else symbol_info.ask
            
            # Prepare close request
            close_request = {
                "action": mt5.TRADE_ACTION_DEAL,
                "symbol": position.symbol,
                "volume": position.volume,
                "type": close_type,
                "position": position.ticket,
                "price": close_price,
                "magic": position.magic,
                "comment": "Close by signal",
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": mt5.ORDER_FILLING_IOC,
            }
            
            # Send close order
            result = mt5.order_send(close_request)
            
            if result.retcode != mt5.TRADE_RETCODE_DONE:
                return {
                    'success': False,
                    'error_code': result.retcode,
                    'error_msg': f'Close failed: {result.comment}'
                }
            
            return {
                'success': True,
                'ticket': result.order,
                'volume': result.volume,
                'price': result.price,
                'profit': position.profit,
                'commission': position.commission,
                'swap': position.swap
            }
            
        except Exception as e:
            logger.error(f"Error closing position: {e}")
            return {
                'success': False,
                'error_code': 5000,
                'error_msg': str(e)
            }
    
    def get_positions(self) -> List[Dict[str, Any]]:
        """Get all open positions."""
        try:
            positions = mt5.positions_get()
            if positions is None:
                return []
            
            result = []
            for pos in positions:
                # Get current price
                tick = mt5.symbol_info_tick(pos.symbol)
                current_price = tick.bid if pos.type == mt5.POSITION_TYPE_BUY else tick.ask
                
                result.append({
                    'ticket': pos.ticket,
                    'symbol': pos.symbol,
                    'volume': pos.volume,
                    'type': 'buy' if pos.type == mt5.POSITION_TYPE_BUY else 'sell',
                    'open_price': pos.price_open,
                    'current_price': current_price,
                    'stop_loss': pos.sl,
                    'take_profit': pos.tp,
                    'profit': pos.profit,
                    'commission': pos.commission,
                    'swap': pos.swap,
                    'comment': pos.comment,
                    'open_time': datetime.fromtimestamp(pos.time).isoformat()
                })
            
            return result
            
        except Exception as e:
            logger.error(f"Error getting positions: {e}")
            return []
    
    def get_account_info(self) -> Dict[str, Any]:
        """Get account information."""
        try:
            account = mt5.account_info()
            if not account:
                return {}
            
            return {
                'balance': account.balance,
                'equity': account.equity,
                'margin': account.margin,
                'free_margin': account.margin_free,
                'currency': account.currency,
                'leverage': account.leverage,
                'connected': True
            }
            
        except Exception as e:
            logger.error(f"Error getting account info: {e}")
            return {'connected': False}

# Initialize MT5 bridge
mt5_bridge = MT5Bridge()

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint."""
    return jsonify({
        'status': 'healthy',
        'connected': mt5_bridge.is_connected(),
        'timestamp': datetime.now().isoformat()
    })

@app.route('/trade', methods=['POST'])
def execute_trade():
    """Execute a trade."""
    try:
        if not mt5_bridge.is_connected():
            return jsonify({
                'success': False,
                'error_code': 5001,
                'error_msg': 'MT5 not connected'
            }), 503
        
        trade_data = request.get_json()
        if not trade_data:
            return jsonify({
                'success': False,
                'error_code': 4000,
                'error_msg': 'Invalid JSON data'
            }), 400
        
        result = mt5_bridge.execute_trade(trade_data)
        status_code = 200 if result['success'] else 400
        
        return jsonify(result), status_code
        
    except Exception as e:
        logger.error(f"Error in execute_trade endpoint: {e}")
        return jsonify({
            'success': False,
            'error_code': 5000,
            'error_msg': str(e)
        }), 500

@app.route('/positions', methods=['GET'])
def get_positions():
    """Get all open positions."""
    try:
        if not mt5_bridge.is_connected():
            return jsonify([]), 503
        
        positions = mt5_bridge.get_positions()
        return jsonify(positions)
        
    except Exception as e:
        logger.error(f"Error in get_positions endpoint: {e}")
        return jsonify([]), 500

@app.route('/account', methods=['GET'])
def get_account():
    """Get account information."""
    try:
        account_info = mt5_bridge.get_account_info()
        return jsonify(account_info)
        
    except Exception as e:
        logger.error(f"Error in get_account endpoint: {e}")
        return jsonify({'connected': False}), 500

if __name__ == '__main__':
    logger.info("Starting MT5 HTTP Bridge Server...")
    
    if not mt5_bridge.is_connected():
        logger.error("Failed to connect to MT5. Please ensure MT5 is running.")
        exit(1)
    
    logger.info("🚀 MT5 Bridge Server starting on http://localhost:8080")
    
    try:
        app.run(host='127.0.0.1', port=8080, debug=False)
    except KeyboardInterrupt:
        logger.info("Shutting down MT5 Bridge Server...")
    finally:
        mt5.shutdown()
        logger.info("MT5 connection closed") 