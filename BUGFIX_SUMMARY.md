# Bug Fix Summary: Numeric Field Overflow

## Issue Description
Error: `pq: numeric field overflow` when processing TradingView webhooks

## Root Cause Analysis
1. **Wrong data types**: `TradingViewWebhook` struct used `float64` instead of `*float64` for optional fields
2. **Zero value handling**: Zero values (`0.0`) were being treated as valid prices instead of nil
3. **Large number validation**: No validation for values exceeding DECIMAL(15,5) database limits
4. **Timestamp issues**: Integer timestamps could cause overflow in certain scenarios

## Applied Fixes

### 1. Database Model Updates
**File**: `internal/database/models.go`

```go
// BEFORE
type TradingViewWebhook struct {
    Price      float64 `json:"price,omitempty"`
    StopLoss   float64 `json:"stop_loss,omitempty"`
    TP1        float64 `json:"tp1,omitempty"`
    TP2        float64 `json:"tp2,omitempty"`
    Timestamp  int64   `json:"timestamp,omitempty"`
}

// AFTER  
type TradingViewWebhook struct {
    Price      *float64 `json:"price,omitempty"`
    Entry      *float64 `json:"entry,omitempty"`      // Alternative field
    StopLoss   *float64 `json:"stop_loss,omitempty"`
    TP1        *float64 `json:"tp1,omitempty"`
    TP2        *float64 `json:"tp2,omitempty"`
    Volume     *float64 `json:"volume,omitempty"`
    Timestamp  string   `json:"timestamp,omitempty"`  // String to handle quotes
}
```

### 2. Enhanced Webhook Parsing
**File**: `internal/signals/processor.go`

- ✅ Added `validatePrice()` function with range checking
- ✅ Maximum value validation: `9999999999.99999` (fits DECIMAL(15,5))
- ✅ Proper nil pointer handling for optional fields
- ✅ TP1/TP2 relationship validation (TP2 > TP1)
- ✅ Support for both `entry` and `price` fields
- ✅ Enhanced logging for debugging

### 3. CloudFlare Worker Validation
**File**: `cf-worker/src/index.js`

- ✅ Comprehensive numeric field validation
- ✅ Range checking for all price fields
- ✅ Volume limit validation (max 100 lots)
- ✅ TP relationship validation
- ✅ Better error messages

### 4. Volume Handling Fix
**File**: `internal/signals/processor.go`

```go
// BEFORE
volume := p.calculatePositionSize(signal.Symbol, tvWebhook.Volume) // Error: *float64 vs float64

// AFTER
requestedVolume := 0.0
if tvWebhook.Volume != nil {
    requestedVolume = *tvWebhook.Volume
}
volume := p.calculatePositionSize(signal.Symbol, requestedVolume)
```

## Validation Rules

### Price Field Validation
- **Range**: 0 < value ≤ 9,999,999,999.99999
- **Precision**: Up to 5 decimal places
- **Optional**: All price fields are optional (use pointers)

### Business Logic Validation
- TP2 must be greater than TP1 (if both present)
- Volume must be ≤ 100 lots
- All prices must be positive numbers

### Field Flexibility
- Supports both `entry` and `price` fields (Pine Script compatibility)
- Optional TP1, TP2 fields (not always required)
- Optional volume field (defaults to risk management settings)

## Testing Scenarios

### ✅ Valid Webhooks
1. Complete signal with TP1 and TP2
2. Basic signal without TP levels  
3. Signal with only TP1
4. Close signal (minimal fields)

### ❌ Invalid Webhooks (Should Fail)
1. Large numbers exceeding database limits
2. TP2 ≤ TP1 (invalid relationship)
3. Negative or zero prices
4. Invalid action types

## Sample Valid Webhook
```json
{
  "ticker": "XAUUSD",
  "action": "buy", 
  "entry": 2650.50,
  "stop_loss": 2645.00,
  "tp1": 2656.00,
  "tp2": 2661.50,
  "volume": 0.01,
  "timestamp": "1704628800"
}
```

## Deployment Steps
1. Build updated Go application: `go build -o trading-engine ./cmd/trading-engine`
2. Deploy updated CloudFlare Worker (if using)
3. Update Pine Script alerts (already compatible)
4. Test with demo account first

## Prevention Measures
- ✅ Input validation at multiple layers (CF Worker + Go Backend)
- ✅ Database constraints and field size limits
- ✅ Comprehensive error logging
- ✅ Type safety with pointer types for optional fields

The system now properly handles optional partial profit fields and prevents numeric overflow errors while maintaining backward compatibility. 