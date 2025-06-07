//+------------------------------------------------------------------+
//|                                           TradingSystemEA.mq5   |
//|                                   MT5 Expert Advisor for Trading System |
//|                                   Alternative to Python Bridge   |
//+------------------------------------------------------------------+
#property copyright "Trading System"
#property link      "https://github.com/your-repo/trading-system"
#property version   "1.00"
#property description "Expert Advisor for receiving webhook signals via HTTP server"

//--- Include files
#include <Trade\Trade.mqh>
#include <JSON.mqh>

//--- Input parameters
input string    WebhookSecret = "your-webhook-secret";       // Webhook secret for validation
input int       ServerPort = 8080;                           // HTTP server port  
input double    DefaultLotSize = 0.01;                       // Default lot size
input int       MagicNumber = 123456;                        // Magic number for trades
input double    MaxDailyLoss = 100.0;                        // Max daily loss in account currency
input int       MaxOpenPositions = 5;                        // Maximum open positions
input bool      EnableRiskManagement = true;                 // Enable risk management
input string    AllowedSymbols = "EURUSD,GBPUSD,USDJPY,XAUUSD"; // Allowed trading symbols

//--- Global variables
CTrade trade;
CPositionInfo position;
CSymbolInfo symbol;
CAccountInfo account;

datetime lastDayReset;
double dailyPnL = 0.0;
int socketHandle = INVALID_HANDLE;

//+------------------------------------------------------------------+
//| Expert initialization function                                   |
//+------------------------------------------------------------------+
int OnInit()
{
    Print("TradingSystemEA: Initializing...");
    
    // Set magic number
    trade.SetExpertMagicNumber(MagicNumber);
    
    // Initialize daily P&L tracking
    lastDayReset = TimeCurrent();
    
    // Start HTTP server
    if(!StartHTTPServer())
    {
        Print("TradingSystemEA: Failed to start HTTP server");
        return INIT_FAILED;
    }
    
    Print("TradingSystemEA: Initialized successfully on port ", ServerPort);
    return INIT_SUCCEEDED;
}

//+------------------------------------------------------------------+
//| Expert deinitialization function                                 |
//+------------------------------------------------------------------+
void OnDeinit(const int reason)
{
    Print("TradingSystemEA: Deinitializing...");
    
    // Close HTTP server
    if(socketHandle != INVALID_HANDLE)
    {
        SocketClose(socketHandle);
        socketHandle = INVALID_HANDLE;
    }
    
    Print("TradingSystemEA: Deinitialized");
}

//+------------------------------------------------------------------+
//| Expert tick function                                             |
//+------------------------------------------------------------------+
void OnTick()
{
    // Check for new HTTP requests
    HandleHTTPRequests();
    
    // Reset daily P&L at start of new day
    CheckDailyReset();
    
    // Update daily P&L
    UpdateDailyPnL();
}

//+------------------------------------------------------------------+
//| Start HTTP server                                               |
//+------------------------------------------------------------------+
bool StartHTTPServer()
{
    socketHandle = SocketCreate();
    if(socketHandle == INVALID_HANDLE)
    {
        Print("TradingSystemEA: Failed to create socket");
        return false;
    }
    
    if(!SocketBind(socketHandle, ServerPort))
    {
        Print("TradingSystemEA: Failed to bind to port ", ServerPort);
        SocketClose(socketHandle);
        return false;
    }
    
    if(!SocketListen(socketHandle))
    {
        Print("TradingSystemEA: Failed to listen on socket");
        SocketClose(socketHandle);
        return false;
    }
    
    Print("TradingSystemEA: HTTP server started on port ", ServerPort);
    return true;
}

//+------------------------------------------------------------------+
//| Handle incoming HTTP requests                                    |
//+------------------------------------------------------------------+
void HandleHTTPRequests()
{
    if(socketHandle == INVALID_HANDLE) return;
    
    int clientSocket = SocketAccept(socketHandle);
    if(clientSocket == INVALID_HANDLE) return;
    
    // Read HTTP request
    string request = "";
    char buffer[1024];
    int received;
    
    while((received = SocketReceive(clientSocket, buffer, 1024)) > 0)
    {
        request += CharArrayToString(buffer, 0, received);
        
        // Check if we have a complete HTTP request
        if(StringFind(request, "\r\n\r\n") >= 0) break;
    }
    
    // Process the request
    string response = ProcessHTTPRequest(request);
    
    // Send response
    SocketSend(clientSocket, response);
    SocketClose(clientSocket);
}

//+------------------------------------------------------------------+
//| Process HTTP request and return response                         |
//+------------------------------------------------------------------+
string ProcessHTTPRequest(string request)
{
    string httpResponse = "";
    
    // Parse request method and path
    string lines[];
    int lineCount = StringSplit(request, '\n', lines);
    if(lineCount == 0) return CreateErrorResponse(400, "Bad Request");
    
    string requestLine = lines[0];
    string parts[];
    int partCount = StringSplit(requestLine, ' ', parts);
    if(partCount < 3) return CreateErrorResponse(400, "Bad Request");
    
    string method = parts[0];
    string path = parts[1];
    
    // Route requests
    if(method == "POST" && path == "/webhook/tradingview")
    {
        return HandleTradingViewWebhook(request);
    }
    else if(method == "GET" && path == "/health")
    {
        return HandleHealthCheck();
    }
    else if(method == "GET" && path == "/positions")
    {
        return HandleGetPositions();
    }
    else if(method == "GET" && path == "/account")
    {
        return HandleGetAccount();
    }
    
    return CreateErrorResponse(404, "Not Found");
}

//+------------------------------------------------------------------+
//| Handle TradingView webhook                                       |
//+------------------------------------------------------------------+
string HandleTradingViewWebhook(string request)
{
    // Extract body from HTTP request
    int bodyStart = StringFind(request, "\r\n\r\n");
    if(bodyStart < 0) return CreateErrorResponse(400, "No body found");
    
    string body = StringSubstr(request, bodyStart + 4);
    
    // TODO: Implement signature validation with WebhookSecret
    
    // Parse JSON
    CJAVal json;
    if(!json.Deserialize(body))
    {
        Print("TradingSystemEA: Failed to parse JSON: ", body);
        return CreateErrorResponse(400, "Invalid JSON");
    }
    
    // Extract signal data
    string ticker = json["ticker"].ToStr();
    string action = json["action"].ToStr();
    double qty = json["qty"].ToDbl();
    double price = json["price"].ToDbl();
    
    // Validate required fields
    if(ticker == "" || action == "")
    {
        return CreateErrorResponse(400, "Missing required fields: ticker, action");
    }
    
    // Process the trading signal
    bool success = ProcessTradingSignal(ticker, action, qty, price);
    
    if(success)
    {
        return CreateSuccessResponse("Signal processed successfully");
    }
    else
    {
        return CreateErrorResponse(500, "Failed to process signal");
    }
}

//+------------------------------------------------------------------+
//| Process trading signal                                           |
//+------------------------------------------------------------------+
bool ProcessTradingSignal(string ticker, string action, double qty, double price)
{
    Print("TradingSystemEA: Processing signal - ", ticker, " ", action, " qty:", qty);
    
    // Validate symbol
    if(!IsSymbolAllowed(ticker))
    {
        Print("TradingSystemEA: Symbol not allowed: ", ticker);
        return false;
    }
    
    // Check risk management
    if(EnableRiskManagement && !CheckRiskLimits())
    {
        Print("TradingSystemEA: Risk limits exceeded");
        return false;
    }
    
    // Select symbol
    if(!symbol.Name(ticker))
    {
        Print("TradingSystemEA: Failed to select symbol: ", ticker);
        return false;
    }
    
    // Determine lot size
    double lotSize = (qty > 0) ? qty : DefaultLotSize;
    
    // Execute trade based on action
    if(action == "BUY" || action == "buy")
    {
        return ExecuteBuy(ticker, lotSize);
    }
    else if(action == "SELL" || action == "sell")
    {
        return ExecuteSell(ticker, lotSize);
    }
    else if(action == "CLOSE" || action == "close")
    {
        return ClosePositions(ticker);
    }
    else if(action == "CLOSE_ALL" || action == "close_all")
    {
        return CloseAllPositions();
    }
    
    Print("TradingSystemEA: Unknown action: ", action);
    return false;
}

//+------------------------------------------------------------------+
//| Execute buy order                                                |
//+------------------------------------------------------------------+
bool ExecuteBuy(string ticker, double lotSize)
{
    if(!symbol.Name(ticker)) return false;
    
    double ask = symbol.Ask();
    double sl = 0; // Stop loss (can be calculated based on risk)
    double tp = 0; // Take profit (can be calculated based on reward)
    
    bool result = trade.Buy(lotSize, ticker, ask, sl, tp, "TradingSystem Buy");
    
    if(result)
    {
        Print("TradingSystemEA: Buy order executed - ", ticker, " Lot:", lotSize, " Price:", ask);
    }
    else
    {
        Print("TradingSystemEA: Buy order failed - ", ticker, " Error:", trade.ResultRetcode());
    }
    
    return result;
}

//+------------------------------------------------------------------+
//| Execute sell order                                               |
//+------------------------------------------------------------------+
bool ExecuteSell(string ticker, double lotSize)
{
    if(!symbol.Name(ticker)) return false;
    
    double bid = symbol.Bid();
    double sl = 0; // Stop loss
    double tp = 0; // Take profit
    
    bool result = trade.Sell(lotSize, ticker, bid, sl, tp, "TradingSystem Sell");
    
    if(result)
    {
        Print("TradingSystemEA: Sell order executed - ", ticker, " Lot:", lotSize, " Price:", bid);
    }
    else
    {
        Print("TradingSystemEA: Sell order failed - ", ticker, " Error:", trade.ResultRetcode());
    }
    
    return result;
}

//+------------------------------------------------------------------+
//| Close positions for specific symbol                             |
//+------------------------------------------------------------------+
bool ClosePositions(string ticker)
{
    bool success = true;
    
    for(int i = PositionsTotal() - 1; i >= 0; i--)
    {
        if(!position.SelectByIndex(i)) continue;
        
        if(position.Symbol() == ticker && position.Magic() == MagicNumber)
        {
            if(!trade.PositionClose(position.Ticket()))
            {
                Print("TradingSystemEA: Failed to close position ", position.Ticket());
                success = false;
            }
            else
            {
                Print("TradingSystemEA: Closed position ", position.Ticket(), " for ", ticker);
            }
        }
    }
    
    return success;
}

//+------------------------------------------------------------------+
//| Close all positions                                             |
//+------------------------------------------------------------------+
bool CloseAllPositions()
{
    bool success = true;
    
    for(int i = PositionsTotal() - 1; i >= 0; i--)
    {
        if(!position.SelectByIndex(i)) continue;
        
        if(position.Magic() == MagicNumber)
        {
            if(!trade.PositionClose(position.Ticket()))
            {
                Print("TradingSystemEA: Failed to close position ", position.Ticket());
                success = false;
            }
            else
            {
                Print("TradingSystemEA: Closed position ", position.Ticket());
            }
        }
    }
    
    return success;
}

//+------------------------------------------------------------------+
//| Check if symbol is allowed for trading                          |
//+------------------------------------------------------------------+
bool IsSymbolAllowed(string ticker)
{
    if(AllowedSymbols == "") return true; // All symbols allowed if empty
    
    string symbols[];
    int count = StringSplit(AllowedSymbols, ',', symbols);
    
    for(int i = 0; i < count; i++)
    {
        if(symbols[i] == ticker) return true;
    }
    
    return false;
}

//+------------------------------------------------------------------+
//| Check risk management limits                                     |
//+------------------------------------------------------------------+
bool CheckRiskLimits()
{
    // Check daily loss limit
    if(dailyPnL <= -MaxDailyLoss)
    {
        Print("TradingSystemEA: Daily loss limit exceeded: ", dailyPnL);
        return false;
    }
    
    // Check maximum open positions
    int openPositions = 0;
    for(int i = 0; i < PositionsTotal(); i++)
    {
        if(!position.SelectByIndex(i)) continue;
        if(position.Magic() == MagicNumber) openPositions++;
    }
    
    if(openPositions >= MaxOpenPositions)
    {
        Print("TradingSystemEA: Maximum open positions exceeded: ", openPositions);
        return false;
    }
    
    return true;
}

//+------------------------------------------------------------------+
//| Handle health check request                                      |
//+------------------------------------------------------------------+
string HandleHealthCheck()
{
    CJAVal json;
    json["status"] = "healthy";
    json["timestamp"] = TimeToString(TimeCurrent());
    json["account"] = account.Login();
    json["balance"] = account.Balance();
    json["equity"] = account.Equity();
    json["daily_pnl"] = dailyPnL;
    
    return CreateJSONResponse(json.Serialize());
}

//+------------------------------------------------------------------+
//| Handle get positions request                                     |
//+------------------------------------------------------------------+
string HandleGetPositions()
{
    CJAVal json, positions;
    
    for(int i = 0; i < PositionsTotal(); i++)
    {
        if(!position.SelectByIndex(i)) continue;
        if(position.Magic() != MagicNumber) continue;
        
        CJAVal pos;
        pos["ticket"] = position.Ticket();
        pos["symbol"] = position.Symbol();
        pos["type"] = EnumToString(position.PositionType());
        pos["volume"] = position.Volume();
        pos["price_open"] = position.PriceOpen();
        pos["price_current"] = position.PriceCurrent();
        pos["profit"] = position.Profit();
        pos["time"] = TimeToString(position.Time());
        
        positions.Add(pos);
    }
    
    json["positions"] = positions;
    return CreateJSONResponse(json.Serialize());
}

//+------------------------------------------------------------------+
//| Handle get account request                                       |
//+------------------------------------------------------------------+
string HandleGetAccount()
{
    CJAVal json;
    json["login"] = account.Login();
    json["balance"] = account.Balance();
    json["equity"] = account.Equity();
    json["margin"] = account.Margin();
    json["free_margin"] = account.FreeMargin();
    json["margin_level"] = account.MarginLevel();
    json["currency"] = account.Currency();
    json["company"] = account.Company();
    json["server"] = account.Server();
    json["daily_pnl"] = dailyPnL;
    
    return CreateJSONResponse(json.Serialize());
}

//+------------------------------------------------------------------+
//| Update daily P&L tracking                                       |
//+------------------------------------------------------------------+
void UpdateDailyPnL()
{
    double totalProfit = 0.0;
    
    for(int i = 0; i < PositionsTotal(); i++)
    {
        if(!position.SelectByIndex(i)) continue;
        if(position.Magic() == MagicNumber)
        {
            totalProfit += position.Profit();
        }
    }
    
    // Add closed trades for today (would need to implement history tracking)
    dailyPnL = totalProfit;
}

//+------------------------------------------------------------------+
//| Check if need to reset daily tracking                           |
//+------------------------------------------------------------------+
void CheckDailyReset()
{
    MqlDateTime now, lastReset;
    TimeToStruct(TimeCurrent(), now);
    TimeToStruct(lastDayReset, lastReset);
    
    if(now.day != lastReset.day)
    {
        dailyPnL = 0.0;
        lastDayReset = TimeCurrent();
        Print("TradingSystemEA: Daily P&L reset for new day");
    }
}

//+------------------------------------------------------------------+
//| Create HTTP success response                                     |
//+------------------------------------------------------------------+
string CreateSuccessResponse(string message)
{
    CJAVal json;
    json["status"] = "success";
    json["message"] = message;
    json["timestamp"] = TimeToString(TimeCurrent());
    
    return CreateJSONResponse(json.Serialize());
}

//+------------------------------------------------------------------+
//| Create HTTP error response                                       |
//+------------------------------------------------------------------+
string CreateErrorResponse(int code, string message)
{
    CJAVal json;
    json["status"] = "error";
    json["code"] = code;
    json["message"] = message;
    json["timestamp"] = TimeToString(TimeCurrent());
    
    string response = "HTTP/1.1 " + IntegerToString(code) + " " + message + "\r\n";
    response += "Content-Type: application/json\r\n";
    response += "Access-Control-Allow-Origin: *\r\n";
    response += "Content-Length: " + IntegerToString(StringLen(json.Serialize())) + "\r\n";
    response += "\r\n";
    response += json.Serialize();
    
    return response;
}

//+------------------------------------------------------------------+
//| Create JSON HTTP response                                        |
//+------------------------------------------------------------------+
string CreateJSONResponse(string jsonBody)
{
    string response = "HTTP/1.1 200 OK\r\n";
    response += "Content-Type: application/json\r\n";
    response += "Access-Control-Allow-Origin: *\r\n";
    response += "Content-Length: " + IntegerToString(StringLen(jsonBody)) + "\r\n";
    response += "\r\n";
    response += jsonBody;
    
    return response;
} 