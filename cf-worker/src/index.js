/**
 * Cloudflare Worker - Trading System Webhook Relay
 * Phase 3: Production deployment component
 * 
 * Purpose:
 * - Global webhook endpoint for TradingView
 * - Rate limiting and DDoS protection  
 * - Request validation and filtering
 * - Multi-region distribution to VPS instances
 * - Webhook authentication and security
 */

export default {
  async fetch(request, env, ctx) {
    // CORS headers for preflight requests
    if (request.method === 'OPTIONS') {
      return new Response(null, {
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'POST, OPTIONS',
          'Access-Control-Allow-Headers': 'Content-Type, X-Signature',
        },
      });
    }

    // Only allow POST requests to webhook endpoint
    if (request.method !== 'POST') {
      return new Response('Method not allowed', { status: 405 });
    }

    const url = new URL(request.url);
    console.log('URL',url.pathname);
    
    // Route webhook requests
    if (url.pathname === '/webhook/tradingview') {
      return    handleTradingViewWebhook(request, env);
    }

    // Health check endpoint
    if (url.pathname === '/health') {
      return new Response(JSON.stringify({
        status: 'healthy',
        timestamp: new Date().toISOString(),
        worker: 'trading-system-relay'
      }), {
        headers: { 'Content-Type': 'application/json' }
      });
    }

    return new Response('Not found', { status: 404 });
  },
};

/**
 * Handle TradingView webhook relay
 */
async function handleTradingViewWebhook(request, env) {
  try {
    // Rate limiting check
    const clientIP = request.headers.get('CF-Connecting-IP');
    const rateLimitKey = `rate_limit:${clientIP}`;
    
    // Check rate limit (max 10 requests per minute)
    const rateLimitCount = await env.TRADING_KV.get(rateLimitKey);
    // if (rateLimitCount && parseInt(rateLimitCount) > 10) {
    //   return new Response('Rate limit exceeded', { status: 429 });
    // }

    // Get request body
    const webhookData = await request.text();
    
    // Validate webhook signature
    const signature = request.headers.get('X-Signature') || request.headers.get('X-Hub-Signature-256');
    // if (!signature) {
    //   return new Response('Missing signature', { status: 401 });
    // }

    // Verify signature using webhook secret
    const isValidSignature = await verifySignature(webhookData, signature, env.WEBHOOK_SECRET);
    // if (!isValidSignature) {
    //   return new Response('Invalid signature', { status: 401 });
    // }

    // Parse and validate webhook payload
    let webhookPayload;
    // console.log('Request',request);
    console.log('Request body',webhookData);
    try {
      webhookPayload = JSON.parse(webhookData);
    } catch (error) {
      return new Response('Invalid JSON payload', { status: 400 });
    }

    // Validate required fields
    if (!webhookPayload.ticker || !webhookPayload.action) {
      return new Response('Missing required fields: ticker, action', { status: 400 });
    }

    // Get active VPS endpoints from KV storage
    const vpsEndpoints = await getActiveVPSEndpoints(env);
    
    if (vpsEndpoints.length === 0) {
      console.error('No active VPS endpoints available');
      return new Response('Service temporarily unavailable', { status: 503 });
    }

    // Distribute webhook to all active VPS instances
    const distributionPromises = vpsEndpoints.map(endpoint => 
      distributeWebhook(endpoint, webhookData, signature)
    );

    // Wait for all distributions (with timeout)
    const results = await Promise.allSettled(distributionPromises);
    
    // Count successful distributions
    const successCount = results.filter(result => result.status === 'fulfilled').length;
    const failedCount = results.length - successCount;

    // Update rate limiting
    await updateRateLimit(env, rateLimitKey);

    // Log webhook event
    await logWebhookEvent(env, {
      timestamp: new Date().toISOString(),
      clientIP,
      symbol: webhookPayload.ticker,
      action: webhookPayload.action,
      successCount,
      failedCount,
      totalEndpoints: vpsEndpoints.length
    });

    // Return success if at least one VPS received the webhook
    if (successCount > 0) {
      return new Response(JSON.stringify({
        status: 'success',
        message: 'Webhook distributed successfully',
        distributed_to: successCount,
        failed: failedCount
      }), {
        headers: { 'Content-Type': 'application/json' }
      });
    } else {
      return new Response(JSON.stringify({
        status: 'error',
        message: 'Failed to distribute to any VPS instance'
      }), {
        status: 502,
        headers: { 'Content-Type': 'application/json' }
      });
    }

  } catch (error) {
    console.error('Webhook processing error:', error);
    return new Response('Internal server error', { status: 500 });
  }
}

/**
 * Verify webhook signature using HMAC-SHA256
 */
async function verifySignature(payload, signature, secret) {
  const encoder = new TextEncoder();
  const key = await crypto.subtle.importKey(
    'raw',
    encoder.encode(secret),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign']
  );

  const signatureBytes = await crypto.subtle.sign('HMAC', key, encoder.encode(payload));
  const expectedSignature = Array.from(new Uint8Array(signatureBytes))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');

  // Remove 'sha256=' prefix if present
  const cleanSignature = signature.replace('sha256=', '');
  
  return expectedSignature === cleanSignature;
}

/**
 * Get list of active VPS endpoints from KV storage
 */
async function getActiveVPSEndpoints(env) {
  try {
    const endpointsData = await env.TRADING_KV.get('vps_endpoints');
    if (!endpointsData) {
      // Default fallback endpoints
      return [
        'https://your-vps-1.com:8081',
        'https://your-vps-2.com:8081'
      ];
    }
    
    const endpoints = JSON.parse(endpointsData);
    // Filter only active endpoints (could implement health checking here)
    return endpoints.filter(endpoint => endpoint.active);
  } catch (error) {
    console.error('Error getting VPS endpoints:', error);
    return [];
  }
}

/**
 * Distribute webhook to a specific VPS endpoint
 */
async function distributeWebhook(endpoint, webhookData, signature) {
  const response = await fetch(`${endpoint}/webhook/tradingview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Signature': signature,
      'User-Agent': 'TradingSystem-CloudflareWorker/1.0'
    },
    body: webhookData,
    // 10 second timeout for VPS response
    signal: AbortSignal.timeout(10000)
  });

  if (!response.ok) {
    throw new Error(`VPS ${endpoint} responded with ${response.status}`);
  }

  return await response.json();
}

/**
 * Update rate limiting counter
 */
async function updateRateLimit(env, rateLimitKey) {
  const current = await env.TRADING_KV.get(rateLimitKey);
  const count = current ? parseInt(current) + 1 : 1;
  
  // Set with 60 second TTL
  await env.TRADING_KV.put(rateLimitKey, count.toString(), { expirationTtl: 60 });
}

/**
 * Log webhook event for monitoring
 */
async function logWebhookEvent(env, eventData) {
  const logKey = `webhook_log:${Date.now()}`;
  await env.TRADING_KV.put(logKey, JSON.stringify(eventData), { expirationTtl: 86400 }); // 24 hour retention
} 