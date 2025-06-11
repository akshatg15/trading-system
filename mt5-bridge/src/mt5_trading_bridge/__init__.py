#!/usr/bin/env python3
"""
MT5 HTTP Bridge Server
Provides REST API interface to MetaTrader 5 for the trading system.
"""

import json
import logging
import time
from datetime import datetime
from typing import Dict, List, Optional, Any, Set
import os
import threading
from dataclasses import dataclass
from queue import Queue

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

@dataclass
class PositionState:
    ticket: int
    symbol: str
    volume: float
    type: int  # mt5.POSITION_TYPE_BUY or mt5.POSITION_TYPE_SELL
    price_open: float
    sl: float
    tp: float
    magic: int
    comment: str
    last_update: datetime

class StateManager:
    def __init__(self):
        self.positions: Dict[int, PositionState] = {}  # ticket -> PositionState
        self.position_lock = threading.Lock()
        self.sync_interval = 5  # seconds
        self.sync_thread = None
        self.stop_event = threading.Event()
    
    def start(self):
        """Start the state synchronization thread."""
        self.sync_thread = threading.Thread(target=self._sync_loop)
        self.sync_thread.daemon = True
        self.sync_thread.start()
    
    def stop(self):
        """Stop the state synchronization thread."""
        self.stop_event.set()
        if self.sync_thread:
            self.sync_thread.join()
    
    def _sync_loop(self):
        """Background thread to sync MT5 positions with our state."""
        while not self.stop_event.is_set():
            try:
                self.sync_positions()
            except Exception as e:
                logger.error(f"Error in position sync loop: {e}")
            time.sleep(self.sync_interval)
    
    def sync_positions(self):
        """Synchronize positions with MT5."""
        try:
            mt5_positions = mt5.positions_get()
            if mt5_positions is None:
                return
            
            current_tickets = set()
            
            with self.position_lock:
                # Update existing positions and track current tickets
                for pos in mt5_positions:
                    current_tickets.add(pos.ticket)
                    if pos.ticket in self.positions:
                        # Update existing position
                        self.positions[pos.ticket] = PositionState(
                            ticket=pos.ticket,
                            symbol=pos.symbol,
                            volume=pos.volume,
                            type=pos.type,
                            price_open=pos.price_open,
                            sl=pos.sl,
                            tp=pos.tp,
                            magic=pos.magic,
                            comment=pos.comment,
                            last_update=datetime.now()
                        )
                    else:
                        # Add new position
                        self.positions[pos.ticket] = PositionState(
                            ticket=pos.ticket,
                            symbol=pos.symbol,
                            volume=pos.volume,
                            type=pos.type,
                            price_open=pos.price_open,
                            sl=pos.sl,
                            tp=pos.tp,
                            magic=pos.magic,
                            comment=pos.comment,
                            last_update=datetime.now()
                        )
                
                # Remove closed positions
                closed_tickets = set(self.positions.keys()) - current_tickets
                for ticket in closed_tickets:
                    del self.positions[ticket]
                    logger.info(f"Removed closed position {ticket}")
        
        except Exception as e:
            logger.error(f"Error syncing positions: {e}")
    
    def get_position(self, ticket: int) -> Optional[PositionState]:
        """Get position by ticket."""
        with self.position_lock:
            return self.positions.get(ticket)
    
    def get_positions_by_magic(self, magic: int) -> List[PositionState]:
        """Get positions by magic number."""
        with self.position_lock:
            return [p for p in self.positions.values() if p.magic == magic]
    
    def get_position_count(self) -> int:
        """Get total number of positions."""
        with self.position_lock:
            return len(self.positions)
    
    def get_all_positions(self) -> List[PositionState]:
        """Get all positions."""
        with self.position_lock:
            return list(self.positions.values())

class MT5Bridge:
    def __init__(self):
        self.connected = False
        self.state_manager = StateManager()
        self.initialize_mt5()
        self.state_manager.start()
    
    def __del__(self):
        """Cleanup on deletion."""
        self.state_manager.stop()
        try:
            mt5.shutdown()
        except:
            pass
    
    def normalize_symbol(self, symbol: str) -> str:
        """Normalize symbol format for MT5 compatibility."""
        # Remove any forward slashes
        symbol = symbol.replace('/', '')
        
        # Try different symbol variations
        variations = [
            symbol,  # Original
            symbol + 'm',  # Add 'm' suffix
            symbol.upper(),  # Uppercase
            symbol.upper() + 'm',  # Uppercase with 'm'
            symbol.lower(),  # Lowercase
            symbol.lower() + 'm'  # Lowercase with 'm'
        ]
        
        # Check which variation exists in MT5
        for sym in variations:
            if mt5.symbol_select(sym, True):
                logger.info(f"Found valid symbol variation: {sym}")
                return sym
        
        # If no variation works, return the original
        return symbol
    
    def ensure_connected(self) -> bool:
        """Ensure MT5 is connected and initialized."""
        try:
            if not self.connected:
                return self.initialize_mt5()
            
            # Check if still connected
            try:
                terminal_info = mt5.terminal_info()
                if not terminal_info or not terminal_info.connected:
                    logger.warning("MT5 connection lost, attempting to reconnect...")
                    mt5.shutdown()
                    return self.initialize_mt5()
            except Exception as e:
                logger.error(f"Error checking terminal info: {e}")
                mt5.shutdown()
                return self.initialize_mt5()
            
            return True
        except Exception as e:
            logger.error(f"Error in ensure_connected: {e}")
            return False
    
    def initialize_mt5(self) -> bool:
        """Initialize MetaTrader 5 connection."""
        try:
            # Shutdown any existing connection
            try:
                mt5.shutdown()
            except:
                pass  # Ignore shutdown errors
            
            # Initialize MT5 with default parameters
            if not mt5.initialize():
                error = mt5.last_error()
                logger.error(f"Failed to initialize MT5: {error}")
                return False
            
            # Check terminal info
            terminal_info = mt5.terminal_info()
            if not terminal_info:
                logger.error("Failed to get terminal info")
                return False
            
            if not terminal_info.connected:
                logger.error("MT5 terminal is not connected")
                return False
            
            # Log detailed terminal info
            logger.info("Terminal Status:")
            logger.info(f"- Connected: {terminal_info.connected}")
            logger.info(f"- Trade allowed: {terminal_info.trade_allowed}")
            logger.info(f"- AutoTrading enabled: {terminal_info.trade_allowed}")
            logger.info(f"- Terminal path: {terminal_info.path}")
            logger.info(f"- Terminal build: {terminal_info.build}")
            logger.info(f"- Terminal connected: {terminal_info.connected}")
            logger.info(f"- Terminal trade allowed: {terminal_info.trade_allowed}")
            # logger.info(f"- Terminal trade mode: {terminal_info.trade_mode}")
            logger.info(f"- Terminal community connection: {terminal_info.community_connection}")
            logger.info(f"- Terminal community balance: {terminal_info.community_balance}")
            logger.info(f"- Terminal data path: {terminal_info.data_path}")
            logger.info(f"- Terminal language: {terminal_info.language}")
            
            if not terminal_info.trade_allowed:
                logger.error("Trading is not allowed in MT5 terminal. Please check:")
                logger.error("1. The 'AutoTrading' button (smiling face) in MT5 toolbar is enabled")
                logger.error("2. 'Allow Algorithmic Trading' is enabled in MT5 settings")
                logger.error("3. The terminal is not in read-only mode")
                return False
            
            # Get account info
            account_info = mt5.account_info()
            if not account_info:
                logger.error("Failed to get account info")
                return False
            
            logger.info(f"Account Info:")
            logger.info(f"- Login: {account_info.login}")
            logger.info(f"- Balance: {account_info.balance} {account_info.currency}")
            logger.info(f"- Server: {account_info.server}")
            # logger.info(f"- Trade mode: {account_info.trade_mode}")
            logger.info(f"- Trade allowed: {account_info.trade_allowed}")
            logger.info(f"- Margin mode: {account_info.margin_mode}")
            logger.info(f"- Leverage: {account_info.leverage}")
            
            # Log available symbols
            try:
                symbols = mt5.symbols_get()
                if symbols:
                    logger.info("Available symbols:")
                    for symbol in symbols:
                        logger.info(f"- {symbol.name}")
                else:
                    logger.error("No symbols available")
            except Exception as e:
                logger.error(f"Error getting symbols: {e}")
            
            self.connected = True
            logger.info("✅ MT5 initialized successfully")
            return True
            
        except Exception as e:
            logger.error(f"Error initializing MT5: {e}")
            return False
    
    def execute_trade(self, trade_data: Dict[str, Any]) -> Dict[str, Any]:
        """Execute a trade in MT5."""
        try:
            # Ensure we're connected
            if not self.ensure_connected():
                return {
                    'success': False,
                    'error_code': 5001,
                    'error_msg': 'MT5 not connected'
                }
            
            # Try different symbol variations
            base_symbol = trade_data.get('symbol', '').replace('/', '')
            symbol_variations = [
                base_symbol,
                base_symbol + 'm',
                base_symbol.upper(),
                base_symbol.upper() + 'm'
            ]
            
            symbol = None
            for sym in symbol_variations:
                if mt5.symbol_select(sym, True):
                    symbol = sym
                    logger.info(f"Found valid symbol: {symbol}")
                    break
            
            if not symbol:
                return {
                    'success': False,
                    'error_code': 4106,
                    'error_msg': f'Symbol {base_symbol} not found (tried variations: {symbol_variations})'
                }
            
            action = trade_data.get('action')  # 'buy', 'sell', 'close'
            volume = float(trade_data.get('volume', 0.01))
            order_type = trade_data.get('order_type', 'market')  # 'market' or 'limit'
            price = trade_data.get('price', 0.0)
            stop_loss = trade_data.get('stop_loss', 0.0)
            take_profit = trade_data.get('take_profit', 0.0)
            tp1 = trade_data.get('tp1', 0.0)  # Not used in simplified version
            tp2 = trade_data.get('tp2', 0.0)  # Not used in simplified version
            comment = trade_data.get('comment', 'AutoTrader')
            magic = trade_data.get('magic', 123456)
            
            # Debug logging for incoming trade data
            logger.info(f"Received trade data:")
            logger.info(f"- Symbol: {trade_data.get('symbol')}")
            logger.info(f"- Action: {action}")
            logger.info(f"- Volume: {volume}")
            logger.info(f"- Price: {price}")
            logger.info(f"- Stop Loss: {stop_loss}")
            logger.info(f"- Take Profit: {take_profit}")
            logger.info(f"- TP1: {tp1}")
            logger.info(f"- TP2: {tp2}")
            logger.info(f"- Magic: {magic}")
            
            # Debug logging
            logger.info(f"Attempting to trade symbol: {symbol}")
            
            # Get symbol info
            symbol_info = mt5.symbol_info(symbol)
            if not symbol_info:
                error = mt5.last_error()
                logger.error(f"Failed to get symbol info for {symbol}: {error}")
                return {
                    'success': False,
                    'error_code': 4106,
                    'error_msg': f'Failed to get symbol info for {symbol}: {error}'
                }
            
            logger.info(f"Symbol info for {symbol}:")
            logger.info(f"- Bid: {symbol_info.bid}")
            logger.info(f"- Ask: {symbol_info.ask}")
            logger.info(f"- Volume min: {symbol_info.volume_min}")
            logger.info(f"- Volume max: {symbol_info.volume_max}")
            logger.info(f"- Trade contract size: {symbol_info.trade_contract_size}")
            logger.info(f"- Point: {symbol_info.point}")
            logger.info(f"- Digits: {symbol_info.digits}")
            logger.info(f"- Stops level: {symbol_info.trade_stops_level}")
            
            # Calculate minimum distance for stops
            min_stop_distance = symbol_info.trade_stops_level * symbol_info.point
            if min_stop_distance == 0:
                min_stop_distance = 10 * symbol_info.point  # Default minimum distance
            
            logger.info(f"Minimum stop distance: {min_stop_distance}")
            
            # Validate volume
            if volume < symbol_info.volume_min:
                volume = symbol_info.volume_min
                logger.info(f"Adjusted volume to minimum: {volume}")
            elif volume > symbol_info.volume_max:
                volume = symbol_info.volume_max
                logger.info(f"Adjusted volume to maximum: {volume}")
            
            # Handle close action
            if action == 'close':
                return self._close_position(trade_data)
            
            # Determine order type and get current price
            if action == 'buy':
                order_type_mt5 = mt5.ORDER_TYPE_BUY if order_type == 'market' else mt5.ORDER_TYPE_BUY_LIMIT
                if order_type == 'market':
                    price = mt5.symbol_info_tick(symbol).ask
                current_price = price
            elif action == 'sell':
                order_type_mt5 = mt5.ORDER_TYPE_SELL if order_type == 'market' else mt5.ORDER_TYPE_SELL_LIMIT
                if order_type == 'market':
                    price = mt5.symbol_info_tick(symbol).bid
                current_price = price
            else:
                return {
                    'success': False,
                    'error_code': 4000,
                    'error_msg': f'Invalid action: {action}'
                }
            
            # Validate and adjust stop loss
            if stop_loss > 0:
                if action == 'buy':
                    if stop_loss >= current_price - min_stop_distance:
                        stop_loss = current_price - min_stop_distance
                        logger.warning(f"Adjusted stop loss to minimum distance: {stop_loss}")
                else:  # sell
                    if stop_loss <= current_price + min_stop_distance:
                        stop_loss = current_price + min_stop_distance
                        logger.warning(f"Adjusted stop loss to minimum distance: {stop_loss}")
            
            # Validate and adjust TP1
            if tp1 > 0:
                if action == 'buy':
                    if tp1 <= current_price + min_stop_distance:
                        tp1 = current_price + min_stop_distance
                        logger.warning(f"Adjusted TP1 to minimum distance: {tp1}")
                else:  # sell
                    if tp1 >= current_price - min_stop_distance:
                        tp1 = current_price - min_stop_distance
                        logger.warning(f"Adjusted TP1 to minimum distance: {tp1}")
            
            # Validate and adjust TP2
            if tp2 > 0:
                if action == 'buy':
                    if tp2 <= current_price + min_stop_distance:
                        tp2 = current_price + min_stop_distance
                        logger.warning(f"Adjusted TP2 to minimum distance: {tp2}")
                else:  # sell
                    if tp2 >= current_price - min_stop_distance:
                        tp2 = current_price - min_stop_distance
                        logger.warning(f"Adjusted TP2 to minimum distance: {tp2}")
            
            # Execute single position as requested (no business logic)
            # Only use take_profit if provided, ignore tp1/tp2 (business logic removed)
            take_profit_level = take_profit if take_profit > 0 else 0
            
            return self._execute_single_position(symbol, action, volume, price, stop_loss, take_profit_level, magic, comment, order_type)
            
        except Exception as e:
            logger.error(f"Error executing trade: {e}")
            return {
                'success': False,
                'error_code': 5000,
                'error_msg': str(e)
            }
    
    def _execute_single_position(self, symbol: str, action: str, volume: float, price: float, stop_loss: float, take_profit: float, magic: int, comment: str, order_type: str = 'market') -> Dict[str, Any]:
        """Execute a single MT5 position or pending order."""
        try:
            # Determine order type and action
            if order_type == 'limit':
                # Limit order (pending)
                if action == 'buy':
                    order_type_mt5 = mt5.ORDER_TYPE_BUY_LIMIT
                else:  # sell
                    order_type_mt5 = mt5.ORDER_TYPE_SELL_LIMIT
                
                action_type = mt5.TRADE_ACTION_PENDING
                # For limit orders, price must be specified
                if price == 0:
                    return {
                        'success': False,
                        'error_code': 4000,
                        'error_msg': 'Price required for limit orders'
                    }
            else:
                # Market order (immediate)
                if action == 'buy':
                    order_type_mt5 = mt5.ORDER_TYPE_BUY
                    if price == 0:
                        price = mt5.symbol_info_tick(symbol).ask
                else:  # sell
                    order_type_mt5 = mt5.ORDER_TYPE_SELL
                    if price == 0:
                        price = mt5.symbol_info_tick(symbol).bid
                
                action_type = mt5.TRADE_ACTION_DEAL
            
            # Prepare order request
            request_dict = {
                "action": action_type,
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
                logger.info(f"Setting stop loss: {stop_loss}")
            if take_profit > 0:
                request_dict["tp"] = take_profit
                logger.info(f"Setting take profit: {take_profit}")
            
            logger.info(f"Single position request: {request_dict}")
            
            # Send order
            result = mt5.order_send(request_dict)
            
            if result.retcode != mt5.TRADE_RETCODE_DONE:
                error = mt5.last_error()
                logger.error(f"Order failed: {result.comment}, Error: {error}")
                return {
                    'success': False,
                    'error_code': result.retcode,
                    'error_msg': f'Order failed: {result.comment}, Error: {error}'
                }
            
            if order_type == 'limit':
                logger.info(f"Pending order created successfully:")
                logger.info(f"- Ticket: {result.order}")
                logger.info(f"- Volume: {result.volume}")
                logger.info(f"- Price: {result.price}")
                
                return {
                    'success': True,
                    'ticket': result.order,
                    'volume': result.volume,
                    'price': result.price,
                    'commission': 0.0,
                    'swap': 0.0,
                    'profit': 0.0
                }
            else:
                logger.info(f"Position created successfully:")
                logger.info(f"- Ticket: {result.order}")
                logger.info(f"- Volume: {result.volume}")
                logger.info(f"- Price: {result.price}")
                
                # Verify position with state manager for market orders
                max_retries = 5
                retry_delay = 1
                position = None
                
                for attempt in range(max_retries):
                    self.state_manager.sync_positions()
                    position = self.state_manager.get_position(result.order)
                    if position:
                        logger.info(f"Position verified in state on attempt {attempt + 1}")
                        break
                    time.sleep(retry_delay)
                
                if not position:
                    logger.error("Position not found after creation")
                    return {
                        'success': False,
                        'error_code': 5000,
                        'error_msg': 'Position not found after creation'
                    }
                
                return {
                    'success': True,
                    'ticket': result.order,
                    'volume': result.volume,
                    'price': result.price,
                    'commission': 0.0,
                    'swap': 0.0,
                    'profit': 0.0
                }
            
        except Exception as e:
            logger.error(f"Error executing single position: {e}")
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
            # Force a sync before returning positions
            self.state_manager.sync_positions()
            
            positions = self.state_manager.get_all_positions()
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
                    'profit': 0.0,  # Will be calculated
                    'commission': 0.0,
                    'swap': 0.0,
                    'comment': pos.comment,
                    'open_time': pos.last_update.isoformat()
                })
            
            return result
            
        except Exception as e:
            logger.error(f"Error getting positions: {e}")
            return []
    
    def get_position_count(self) -> int:
        """Get the actual number of open positions in MT5."""
        try:
            # Force a sync before returning count
            self.state_manager.sync_positions()
            return self.state_manager.get_position_count()
        except Exception as e:
            logger.error(f"Error getting position count: {e}")
            return 0
    
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

    def get_orders(self) -> List[Dict[str, Any]]:
        """Get all pending orders."""
        try:
            orders = mt5.orders_get()
            if orders is None:
                return []
            
            result = []
            for order in orders:
                result.append({
                    'ticket': order.ticket,
                    'symbol': order.symbol,
                    'volume': order.volume_current,
                    'type': self._order_type_to_string(order.type),
                    'price': order.price_open,
                    'stop_loss': order.sl,
                    'take_profit': order.tp,
                    'comment': order.comment,
                    'open_time': datetime.fromtimestamp(order.time_setup).isoformat()
                })
            
            return result
            
        except Exception as e:
            logger.error(f"Error getting orders: {e}")
            return []

    def get_order_count(self) -> int:
        """Get the number of pending orders."""
        try:
            orders = mt5.orders_get()
            return len(orders) if orders else 0
        except Exception as e:
            logger.error(f"Error getting order count: {e}")
            return 0

    def _order_type_to_string(self, order_type: int) -> str:
        """Convert MT5 order type to string."""
        type_map = {
            mt5.ORDER_TYPE_BUY: "buy",
            mt5.ORDER_TYPE_SELL: "sell",
            mt5.ORDER_TYPE_BUY_LIMIT: "buy_limit",
            mt5.ORDER_TYPE_SELL_LIMIT: "sell_limit",
            mt5.ORDER_TYPE_BUY_STOP: "buy_stop",
            mt5.ORDER_TYPE_SELL_STOP: "sell_stop",
            mt5.ORDER_TYPE_BUY_STOP_LIMIT: "buy_stop_limit",
            mt5.ORDER_TYPE_SELL_STOP_LIMIT: "sell_stop_limit"
        }
        return type_map.get(order_type, f"unknown_{order_type}")

    def modify_position(self, modify_data: Dict[str, Any]) -> Dict[str, Any]:
        """Modify an existing position with TP/SL levels or create partial closing orders."""
        try:
            # Ensure we're connected
            if not self.ensure_connected():
                return {
                    'success': False,
                    'error_code': 5001,
                    'error_msg': 'MT5 not connected'
                }
            
            position_ticket = modify_data.get('position_ticket')
            symbol = modify_data.get('symbol')
            take_profit = modify_data.get('take_profit', 0.0)
            stop_loss = modify_data.get('stop_loss', 0.0) 
            partial_volume = modify_data.get('partial_volume', 0.0)
            tp_type = modify_data.get('tp_type', '')
            
            if not position_ticket:
                return {
                    'success': False,
                    'error_code': 4000,
                    'error_msg': 'Position ticket is required'
                }
            
            # Find the position
            positions = mt5.positions_get(ticket=position_ticket)
            if not positions:
                return {
                    'success': False,
                    'error_code': 4051,
                    'error_msg': f'Position {position_ticket} not found'
                }
            
            position = positions[0]
            
            # For partial TP orders, create a limit order that will close part of the position
            if partial_volume > 0 and take_profit > 0:
                return self._create_partial_tp_order(position, take_profit, partial_volume, tp_type)
            
            # For full position modification, modify TP/SL levels
            return self._modify_position_levels(position, take_profit, stop_loss)
            
        except Exception as e:
            logger.error(f"Error modifying position: {e}")
            return {
                'success': False,
                'error_code': 5000,
                'error_msg': str(e)
            }
    
    def _create_partial_tp_order(self, position, take_profit: float, volume: float, tp_type: str) -> Dict[str, Any]:
        """Create a limit order to close part of a position at TP level."""
        try:
            # Determine the opposite order type for closing
            if position.type == mt5.POSITION_TYPE_BUY:
                order_type = mt5.ORDER_TYPE_SELL_LIMIT
                action = "sell"
            else:
                order_type = mt5.ORDER_TYPE_BUY_LIMIT  
                action = "buy"
            
            # Validate volume doesn't exceed position volume
            if volume > position.volume:
                volume = position.volume
                logger.warning(f"Adjusted TP volume to position volume: {volume}")
            
            # Get symbol info for validation
            symbol_info = mt5.symbol_info(position.symbol)
            if not symbol_info:
                return {
                    'success': False,
                    'error_code': 4106,
                    'error_msg': f'Failed to get symbol info for {position.symbol}'
                }
            
            # Validate TP price against current market
            current_price = mt5.symbol_info_tick(position.symbol).bid if position.type == mt5.POSITION_TYPE_BUY else mt5.symbol_info_tick(position.symbol).ask
            min_distance = symbol_info.trade_stops_level * symbol_info.point
            
            if position.type == mt5.POSITION_TYPE_BUY:
                if take_profit <= current_price + min_distance:
                    take_profit = current_price + min_distance
                    logger.warning(f"Adjusted TP price to minimum distance: {take_profit}")
            else:
                if take_profit >= current_price - min_distance:
                    take_profit = current_price - min_distance
                    logger.warning(f"Adjusted TP price to minimum distance: {take_profit}")
            
            # Create limit order request
            request_dict = {
                "action": mt5.TRADE_ACTION_PENDING,
                "symbol": position.symbol,
                "volume": volume,
                "type": order_type,
                "price": take_profit,
                "magic": position.magic,
                "comment": f"TP_{tp_type}_{position.ticket}",
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": mt5.ORDER_FILLING_IOC,
            }
            
            logger.info(f"Creating partial TP order: {request_dict}")
            
            # Send the order
            result = mt5.order_send(request_dict)
            
            if result.retcode != mt5.TRADE_RETCODE_DONE:
                error = mt5.last_error()
                logger.error(f"TP order failed: {result.comment}, Error: {error}")
                return {
                    'success': False,
                    'error_code': result.retcode,
                    'error_msg': f'TP order failed: {result.comment}, Error: {error}'
                }
            
            logger.info(f"Partial TP order created successfully:")
            logger.info(f"- TP Order Ticket: {result.order}")
            logger.info(f"- Volume: {result.volume}")
            logger.info(f"- TP Price: {result.price}")
            
            return {
                'success': True,
                'tp_order_ticket': result.order,
                'volume': result.volume,
                'price': result.price,
                'commission': 0.0
            }
            
        except Exception as e:
            logger.error(f"Error creating partial TP order: {e}")
            return {
                'success': False,
                'error_code': 5000,
                'error_msg': str(e)
            }
    
    def _modify_position_levels(self, position, take_profit: float, stop_loss: float) -> Dict[str, Any]:
        """Modify the TP/SL levels of an existing position."""
        try:
            # Use the existing TP/SL if not provided
            new_tp = take_profit if take_profit > 0 else position.tp
            new_sl = stop_loss if stop_loss > 0 else position.sl
            
            # Create modification request
            request_dict = {
                "action": mt5.TRADE_ACTION_SLTP,
                "symbol": position.symbol,
                "position": position.ticket,
                "sl": new_sl,
                "tp": new_tp,
                "magic": position.magic,
                "comment": f"Modify_{position.ticket}",
                "type_time": mt5.ORDER_TIME_GTC,
                "type_filling": mt5.ORDER_FILLING_IOC,
            }
            
            logger.info(f"Modifying position levels: {request_dict}")
            
            # Send the modification
            result = mt5.order_send(request_dict)
            
            if result.retcode != mt5.TRADE_RETCODE_DONE:
                error = mt5.last_error()
                logger.error(f"Position modification failed: {result.comment}, Error: {error}")
                return {
                    'success': False,
                    'error_code': result.retcode,
                    'error_msg': f'Position modification failed: {result.comment}, Error: {error}'
                }
            
            logger.info(f"Position levels modified successfully for ticket: {position.ticket}")
            
            return {
                'success': True,
                'tp_order_ticket': 0,  # No separate order ticket for SL/TP modification
                'commission': 0.0
            }
            
        except Exception as e:
            logger.error(f"Error modifying position levels: {e}")
            return {
                'success': False,
                'error_code': 5000,
                'error_msg': str(e)
            }

# Initialize MT5 bridge
mt5_bridge = MT5Bridge()

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint."""
    return jsonify({
        'status': 'healthy',
        'connected': mt5_bridge.ensure_connected(),
        'timestamp': datetime.now().isoformat()
    })

@app.route('/trade', methods=['POST'])
def execute_trade():
    """Execute a trade."""
    try:
        if not mt5_bridge.ensure_connected():
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
    positions = mt5_bridge.get_positions()
    return jsonify(positions)

@app.route('/position/modify', methods=['POST'])
def modify_position():
    """Modify an existing position with TP/SL levels."""
    try:
        if not mt5_bridge.ensure_connected():
            return jsonify({
                'success': False,
                'error_code': 5001,
                'error_msg': 'MT5 not connected'
            }), 503
        
        modify_data = request.get_json()
        if not modify_data:
            return jsonify({
                'success': False,
                'error_code': 4000,
                'error_msg': 'Invalid JSON data'
            }), 400
        
        result = mt5_bridge.modify_position(modify_data)
        status_code = 200 if result['success'] else 400
        
        return jsonify(result), status_code
        
    except Exception as e:
        logger.error(f"Error in modify_position endpoint: {e}")
        return jsonify({
            'success': False,
            'error_code': 5000,
            'error_msg': str(e)
        }), 500

@app.route('/positions/count', methods=['GET'])
def get_position_count():
    """Get the number of open positions."""
    count = mt5_bridge.get_position_count()
    return jsonify({
        'count': count,
        'timestamp': datetime.now().isoformat()
    })

@app.route('/account', methods=['GET'])
def get_account():
    """Get account information."""
    try:
        account_info = mt5_bridge.get_account_info()
        return jsonify(account_info)
        
    except Exception as e:
        logger.error(f"Error in get_account endpoint: {e}")
        return jsonify({'connected': False}), 500

@app.route('/orders', methods=['GET'])
def get_orders():
    """Get all pending orders."""
    orders = mt5_bridge.get_orders()
    return jsonify(orders)

@app.route('/order-count', methods=['GET'])
def get_order_count():
    """Get the number of pending orders."""
    count = mt5_bridge.get_order_count()
    return jsonify({
        'count': count,
        'timestamp': datetime.now().isoformat()
    })

def main():
    """Main entry point for the MT5 bridge server."""
    try:
        bridge = MT5Bridge()
        if not bridge.ensure_connected():
            logger.error("Failed to connect to MT5. Please check your MT5 terminal is running.")
            return
        
        host = os.getenv('MT5_HOST', 'localhost')
        port = int(os.getenv('MT5_PORT', '8080'))
        
        logger.info(f"Starting MT5 Bridge server on {host}:{port}")
        app.run(host=host, port=port)
    except Exception as e:
        logger.error(f"Error in main: {e}")

if __name__ == "__main__":
    main() 