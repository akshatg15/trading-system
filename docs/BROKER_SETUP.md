# Broker Setup Guide - MT5 Trading System

This guide explains how to connect your trading system to **any broker** that supports MetaTrader 5.

## 🏦 Supported Brokers

The system works with **ANY MT5-compatible broker**, including:

- **Exness** (Popular choice for algorithmic trading)
- **IC Markets** 
- **Pepperstone**
- **FXCM**
- **Admiral Markets**
- **XM**
- **OANDA** (MT5 accounts)
- **IG Markets**
- **Plus500** (MT5)
- **Any other MT5 broker**

## 🔧 Broker Connection Process

### Step 1: Choose Your Broker

**Recommended for Algorithmic Trading:**
- **Exness**: Low spreads, high leverage, good API support
- **IC Markets**: Excellent execution, low latency
- **Pepperstone**: Good for scalping, tight spreads

**Key Criteria:**
- ✅ **MT5 Support** (mandatory)
- ✅ **Algorithmic Trading Allowed**
- ✅ **Low Latency** execution
- ✅ **Competitive Spreads**
- ✅ **Good Customer Support**
- ✅ **Regulatory Compliance**

### Step 2: Open Trading Account

1. **Visit broker's website** (e.g., exness.com)
2. **Complete KYC verification** (ID, address proof)
3. **Choose account type**: 
   - Raw Spread / Zero accounts (lowest costs)
   - Standard accounts (if Raw unavailable)
4. **Fund your account** (minimum amounts vary)

### Step 3: Download & Install MT5

**For Exness Example:**
```bash
# Download MT5 from Exness
# Visit: https://www.exness.com/trading-platforms/metatrader-5/

# Or download generic MT5
# Visit: https://www.metatrader5.com/en/download
```

**Installation:**
1. Download MT5 installer for Windows
2. Run installer on your Windows VPS
3. Complete installation with default settings

### Step 4: Connect to Broker

**In MT5 Terminal:**
```
File → Login to Trade Account
```

**Enter your broker details:**
- **Login**: Your account number (from broker)
- **Password**: Your trading password  
- **Server**: Your broker's MT5 server

**Exness Server Examples:**
- `Exness-MT5Trial` (demo)
- `Exness-MT5Real` (live)
- `ExnessEU-MT5Real` (EU clients)

**Other Broker Servers:**
- IC Markets: `ICMarketsSC-MT5`
- Pepperstone: `Pepperstone-MT5`
- XM: `XMGlobal-MT5`

### Step 5: Enable Algorithmic Trading

**In MT5:**
```
Tools → Options → Expert Advisors

✅ Allow automated trading
✅ Allow DLL imports  
✅ Allow imports of external experts
```

**Security Settings:**
- Allow listed URLs only: `http://localhost:8080`
- Or allow all URLs for development

### Step 6: Test Connection

**Verify MT5 is working:**
```bash
# Start MT5 bridge
cd trading-system/mt5-bridge
python mt5_bridge.py

# Test connection
curl http://localhost:8080/health
curl http://localhost:8080/account
```

## 📊 Account Configuration

### Account Types by Broker

**Exness:**
- **Raw Spread**: 0 spread + commission (best for algo trading)
- **Zero**: Similar to Raw Spread
- **Standard**: Fixed spreads, no commission

**IC Markets:**
- **Raw Spread**: 0.1 pip average + $3.50 commission
- **Standard**: 1.0 pip average, no commission

**Pepperstone:**
- **Razor**: 0.0 pip spread + commission
- **Standard**: 1.6 pip average spread

### Recommended Settings

**For High-Frequency Trading:**
```
Account Type: Raw Spread / Zero / Razor
Leverage: 1:500 or 1:1000
Base Currency: USD (for easier P&L calculation)
```

**For Conservative Trading:**
```
Account Type: Standard
Leverage: 1:100 or 1:200  
Base Currency: Your local currency
```

## 🔐 Security & Risk Management

### Broker-Level Protection

**Choose brokers with:**
- ✅ **Regulatory license** (FCA, ASIC, CySEC, etc.)
- ✅ **Segregated funds** (client money protection)
- ✅ **Negative balance protection**
- ✅ **Insurance coverage** (optional but preferred)

**Exness Protections:**
- FCA regulated
- Up to €20,000 compensation
- Negative balance protection
- Segregated client funds

### Account Security

```bash
# Use strong passwords
# Enable 2FA if available  
# Regular password rotation
# VPS-only access (no personal computers)
```

### Risk Controls

**In your .env file:**
```bash
# Conservative settings for live trading
RISK_MAX_DAILY_LOSS=100.00
RISK_MAX_POSITION_SIZE=10.0  # 0.10 lots = $10 per pip
RISK_MAX_OPEN_POSITIONS=2
```

## 🚀 Production Deployment

### VPS Requirements

**Windows VPS Specifications:**
- **OS**: Windows Server 2019/2022 or Windows 10/11
- **RAM**: 4GB minimum (8GB recommended)
- **CPU**: 2 cores minimum (4 cores for multiple brokers)
- **Storage**: 50GB SSD
- **Network**: 1Gbps connection, <50ms latency to broker

**Recommended VPS Providers:**
- **AWS EC2** (Windows instances)
- **Azure Virtual Machines**
- **Google Cloud Compute Engine**
- **Vultr** (cheaper alternative)
- **DigitalOcean** (if Windows support available)

### Multi-Broker Setup

**For advanced users - connect multiple brokers:**

```python
# In mt5_bridge.py - extend for multiple connections
brokers = {
    'exness': {'login': 12345, 'server': 'Exness-MT5Real'},
    'ic_markets': {'login': 67890, 'server': 'ICMarketsSC-MT5'},
}

# Route trades based on symbol or strategy
def execute_trade_multi_broker(symbol, action):
    if symbol.startswith('XAU'):  # Gold trades to Exness
        return execute_on_broker('exness', trade_data)
    else:  # Forex trades to IC Markets
        return execute_on_broker('ic_markets', trade_data)
```

## 📞 Broker Support & Troubleshooting

### Common Issues

**Connection Problems:**
1. **Wrong server name**: Check broker's official MT5 server list
2. **Firewall blocking**: Allow MT5.exe through Windows firewall
3. **Account suspended**: Contact broker support
4. **Wrong credentials**: Verify login/password

**Trading Issues:**
1. **"Trade disabled"**: Enable auto-trading in MT5
2. **"Not enough money"**: Check account balance and margin
3. **"Invalid volume"**: Check minimum lot size (usually 0.01)
4. **"Market closed"**: Check trading hours for your symbols

### Broker Support Contacts

**Exness:**
- 24/7 Live Chat
- Email: support@exness.com
- Phone: +44 20 7633 5430

**IC Markets:**
- 24/5 Support
- Email: support@icmarkets.com
- Phone: +61 2 8002 7202

### Getting Help

**For technical trading issues:**
1. Check MT5 terminal logs (`File → Open Data Folder → Logs`)
2. Review Python bridge logs
3. Test with demo account first
4. Contact broker's technical support
5. Use broker's API documentation

**For system integration:**
1. Check our trading system logs
2. Test MT5 bridge connectivity
3. Verify webhook delivery
4. Monitor position synchronization

## 💡 Pro Tips

### Optimization

1. **Use Raw Spread accounts** for algorithmic trading
2. **Deploy VPS in same region** as broker's data center
3. **Test with demo account** before going live
4. **Monitor latency** - aim for <50ms to broker
5. **Use multiple brokers** for redundancy

### Cost Management

```bash
# Calculate costs for your trading volume
# Exness Raw Spread example:
# Commission: $3.50 per lot per side
# 1 lot EURUSD trade = $7.00 total commission
# 0.01 lot trade = $0.07 total commission
```

### Monitoring

**Set up alerts for:**
- Account balance changes
- Margin call warnings  
- Connection issues
- Failed trade executions
- Unusual trading activity

This setup allows you to trade with **any MT5 broker worldwide** while maintaining full control over your trading system. 