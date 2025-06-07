# Cloudflare Worker - Trading System Webhook Relay

This Cloudflare Worker provides a **global webhook relay service** for the MT5 Trading System, implementing **Phase 3** of the production deployment architecture.

## 🎯 Purpose

- **Global Edge Distribution**: Deploy webhook endpoint globally via Cloudflare's edge network
- **DDoS Protection**: Built-in protection against attacks and spam
- **Rate Limiting**: Prevent abuse with per-IP rate limiting  
- **Multi-VPS Distribution**: Relay webhooks to multiple VPS instances for redundancy
- **Security**: Webhook signature validation and request filtering
- **Monitoring**: Comprehensive logging and performance tracking

## 🏗️ Architecture

```
TradingView → Cloudflare Worker → [VPS-1, VPS-2, VPS-N] → MT5 Bridge → MT5
```

### Why Cloudflare Worker?

1. **Global Availability**: 300+ edge locations worldwide
2. **Low Latency**: Process webhooks at the edge closest to TradingView
3. **High Reliability**: 99.99% uptime SLA
4. **Auto-scaling**: Handle any webhook volume
5. **Cost Effective**: Only pay for actual usage
6. **Security**: Built-in DDoS protection and WAF

## 🚀 Deployment

### Prerequisites

1. **Cloudflare Account** with Workers plan
2. **Domain** managed by Cloudflare (optional but recommended)
3. **Node.js** 18+ and npm
4. **Wrangler CLI** installed globally

### Setup

```bash
# Install Wrangler CLI
npm install -g wrangler

# Navigate to cf-worker directory
cd trading-system/cf-worker

# Install dependencies
npm install

# Login to Cloudflare
wrangler login

# Create KV namespaces
npm run setup

# Set secrets
wrangler secret put WEBHOOK_SECRET
# Enter your webhook secret when prompted
```

### Configuration

1. **Update wrangler.toml**:
   - Replace `your-kv-namespace-id` with actual KV namespace IDs
   - Update domain routes to match your domain
   - Configure custom domains if needed

2. **Configure VPS Endpoints**:
```bash
# Set VPS endpoints in KV storage
wrangler kv:key put --binding=TRADING_KV "vps_endpoints" '[
  {"url": "https://your-vps-1.com:8081", "active": true},
  {"url": "https://your-vps-2.com:8081", "active": true}
]'
```

### Deploy

```bash
# Deploy to staging
npm run deploy:staging

# Deploy to production  
npm run deploy:production

# Test deployment
curl https://your-worker-domain.com/health
```

## 🔧 Features

### Rate Limiting
- **10 requests per minute** per IP address
- Automatic cleanup with TTL
- Returns `429 Too Many Requests` when exceeded

### Webhook Validation
- **HMAC-SHA256** signature verification
- **Required fields** validation (ticker, action)
- **JSON payload** validation

### Multi-VPS Distribution
- **Parallel distribution** to all active VPS instances
- **Timeout handling** (10 seconds per VPS)
- **Partial success** handling (succeeds if any VPS responds)
- **Health checking** integration ready

### Monitoring & Logging
- **Request logging** with timestamps and IP addresses
- **Distribution metrics** (success/failure counts)
- **24-hour log retention** in KV storage
- **Real-time monitoring** via Cloudflare dashboard

## 🔗 Integration

### TradingView Configuration

Update your TradingView webhook URL to point to the Cloudflare Worker:

```
https://trading-api.yourdomain.com/webhook/tradingview
```

### VPS Configuration

Ensure your VPS instances are configured to accept webhooks from the Cloudflare Worker:

1. **Firewall**: Allow HTTPS traffic on port 8081
2. **SSL Certificate**: Use valid SSL certificates (Let's Encrypt recommended)
3. **Domain**: Configure proper domain names for each VPS
4. **Health Checks**: Implement `/health` endpoint on each VPS

## 📊 Monitoring

### Cloudflare Dashboard
- **Real-time metrics**: Request volume, error rates, latency
- **Geographic distribution**: See where requests are coming from
- **Performance insights**: Cache hit rates, CPU usage

### Worker Analytics
```bash
# View real-time logs
npm run tail

# View KV storage
wrangler kv:key list --binding=TRADING_KV
```

### Custom Monitoring
- **Webhook logs**: Stored in KV with key pattern `webhook_log:*`
- **Rate limit data**: Stored with TTL in KV storage
- **VPS endpoints**: Managed in KV storage for dynamic updates

## 🛡️ Security

### Built-in Protection
- **DDoS protection** via Cloudflare
- **Rate limiting** per IP address
- **Request size limits** (automatic)
- **Geographic filtering** (optional)

### Authentication
- **HMAC-SHA256** webhook signatures
- **Secret validation** using environment variables
- **Header-based authentication** support

### Best Practices
1. **Rotate webhook secrets** regularly
2. **Monitor unusual traffic** patterns
3. **Use HTTPS only** for all endpoints
4. **Implement IP whitelisting** if needed
5. **Regular security audits** of worker code

## 🔄 Maintenance

### Updating VPS Endpoints
```bash
# Add new VPS
wrangler kv:key put --binding=TRADING_KV "vps_endpoints" '[
  {"url": "https://vps-1.com:8081", "active": true},
  {"url": "https://vps-2.com:8081", "active": true},
  {"url": "https://vps-3.com:8081", "active": true}
]'

# Disable VPS for maintenance
wrangler kv:key put --binding=TRADING_KV "vps_endpoints" '[
  {"url": "https://vps-1.com:8081", "active": false},
  {"url": "https://vps-2.com:8081", "active": true}
]'
```

### Scaling Considerations
- **KV storage limits**: 100GB per namespace
- **CPU time limits**: 30 seconds per request
- **Memory limits**: 128MB per request
- **Request limits**: 100,000 requests per day (free tier)

## 🚨 Troubleshooting

### Common Issues

1. **502 Bad Gateway**: Check VPS endpoints are accessible
2. **429 Rate Limited**: Increase rate limits or check for abuse
3. **401 Unauthorized**: Verify webhook secret configuration
4. **Timeout errors**: Check VPS response times

### Debugging

```bash
# View logs in real-time
wrangler tail

# Test locally
npm run test

# Check KV storage
wrangler kv:key list --binding=TRADING_KV
```

## 📈 Performance

### Expected Performance
- **Latency**: < 50ms response time globally
- **Throughput**: 1000+ requests per second
- **Availability**: 99.99% uptime
- **Global reach**: 300+ edge locations

### Optimization Tips
1. **Minimize KV reads** in hot paths
2. **Use parallel requests** for VPS distribution
3. **Implement caching** for static configuration
4. **Monitor CPU usage** and optimize heavy operations 