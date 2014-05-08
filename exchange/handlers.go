package exchange

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/auth"
    "github.com/jaekwon/GoLLRB/llrb"
    //"github.com/davecgh/go-spew/spew"
    "net/http"
    "time"
    "fmt"
)

// Simplified order for orderbook API
type SOrder struct {
    Amount      uint64  `json:"a"`
    Price       string  `json:"p"`
}

// Helper for getting the market from request param
func GetParamMarket(r *http.Request, paramName string) *Market {
    mName := GetParam(r, paramName)
    market := Markets[mName]
    if market == nil { ReturnJSON(API_INVALID_PARAM, "Market "+mName+" does not exist") }
    return market
}

func MarketsHandler(w http.ResponseWriter, r *http.Request) {
    type marketInfo struct {
        Coin        string  `json:"coin"`
        BasisCoin   string  `json:"basisCoin"`
        Last        float64 `json:"last"`
        BestBid     float64 `json:"bestBid"`
        BestAsk     float64 `json:"bestAsk"`
    }
    var infos = []marketInfo{}
    for _, marketName := range MarketNames {
        market := Markets[marketName]
        infos = append(infos, marketInfo{
            Coin:       market.Coin,
            BasisCoin:  market.BasisCoin,
            Last:       market.PriceLogger.LastPrice(),
            BestBid:    market.BestBidPrice(),
            BestAsk:    market.BestAskPrice(),
        })
    }
    ReturnJSON(API_OK, infos)
}

func OrderBookHandler(w http.ResponseWriter, r *http.Request) {
    market := GetParamMarket(r, "market")

    // Take a snapshot of each so we don't return overlapping bids/asks.
    mBids, mAsks := market.Bids.Snapshot(), market.Asks.Snapshot()

    bids, asks := []*SOrder{}, []*SOrder{}
    var curSOrder *SOrder

    mBids.AscendGreaterOrEqual(mBids.Min(), func(i llrb.Item) bool {
        bid := i.(*Order)
        sfAmount := uint64(0)
        sfPrice := F64ToS(bid.Price, 5)
        if bid.Amount != 0 {
            sfAmount = bid.Amount - bid.Filled
        } else {
            sfAmount = uint64(float64(bid.BasisAmount - bid.BasisFilled) * bid.Price)
        }
        if sfAmount == uint64(0) { return true }
        if curSOrder == nil || curSOrder.Price != sfPrice {
            curSOrder = &SOrder{sfAmount, sfPrice}
            bids = append(bids, curSOrder)
        } else {
            curSOrder.Amount += sfAmount
        }
        return true
    })
    curSOrder = nil
    mAsks.AscendGreaterOrEqual(mAsks.Min(), func(i llrb.Item) bool {
        ask := i.(*Order)
        sfPrice := F64ToS(ask.Price, 5)
        sfAmount := ask.Amount - ask.Filled
        if sfAmount == uint64(0) { return true }
        if curSOrder == nil || curSOrder.Price != sfPrice {
            curSOrder = &SOrder{sfAmount, sfPrice}
            asks = append(asks, curSOrder)
        } else {
            curSOrder.Amount += sfAmount
        }
        return true
    })

    res := map[string]interface{}{
        "bids": bids,
        "asks": asks,
    }

    ReturnJSON(API_OK, res)
}

func AddOrderHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    market :=           GetParamMarket(r, "market")
    orderType :=        GetParamRegexp(r, "order_type",   RE_ORDER_TYPE, true)
    amount, _ :=        GetParamUint64Safe(r, "amount")
    basisAmount, _ :=   GetParamUint64Safe(r, "basis_amount")
    price :=            GetParamFloat64(r, "price")

    c := Config.GetCoin(market.Coin)
    bc := Config.GetCoin(market.BasisCoin)

    // Validation
    if amount == 0 && basisAmount == 0 {
        ReturnJSON(API_INVALID_PARAM,
            fmt.Sprintf("Please enter a valid order amount"))
    }
    if price <= 0 {
        ReturnJSON(API_INVALID_PARAM,
            fmt.Sprintf("Please enter a valid order price"))
    }

    // Round price to 5 significant figures
    price = F64ToF(price, 5)

    // Ensure that trades aren't dust.
    if amount > 0 && amount < c.MinTrade {
        ReturnJSON(API_INVALID_PARAM,
            fmt.Sprintf("Minimum order amount is %v %v", I64ToF64(int64(c.MinTrade)), market.Coin))
    }
    if basisAmount > 0 && basisAmount < bc.MinTrade {
        ReturnJSON(API_INVALID_PARAM,
            fmt.Sprintf("Minimum order amount is %v %v", I64ToF64(int64(bc.MinTrade)), market.BasisCoin))
    }

    if orderType == "A" && amount == 0 {
        amount = uint64(float64(basisAmount) / float64(price) + 0.5)
    } else if orderType == "B" && basisAmount == 0 {
        basisAmount = uint64(float64(amount) * float64(price) + 0.5)
    }

    order := &Order{
        Type:           orderType,
        UserId:         user.Id,
        Coin:           market.Coin,
        Amount:         amount,
        BasisCoin:      market.BasisCoin,
        BasisAmount:    basisAmount,
        Price:          price,
    }
    AddOrder(order)

    ReturnJSON(API_OK, order)
}

func CancelOrderHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    id := GetParamInt64(r, "id")

    order := LoadOrder(id)
    if order == nil { ReturnJSON(API_INVALID_PARAM, "Order with that id does not exist") }

    CancelOrder(order)

    ReturnJSON(API_OK, "CANCELED")
}

func GetPendingOrdersHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    market := GetParamMarket(r, "market")
    orders := LoadPendingOrdersByUser(user.Id, market.BasisCoin, market.Coin)
    ReturnJSON(API_OK, orders)
}

func TradeHistoryHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    // TODO
}

func PriceLogHandler(w http.ResponseWriter, r *http.Request) {
    market := GetParamMarket(r, "market")
    start  := GetParamInt64(r, "start")
    end    := GetParamInt64(r, "end")

    if end == 0 || end < int64(0) { end = time.Now().Unix() + end }
    if start == 0 { ReturnJSON(API_INVALID_PARAM, "Parameter 'start' cannot be 0") }
    if start < 0 { start = time.Now().Unix() + start }

    // TODO: automatically adjust interval based on start & end.
    plogs := market.PriceLogger.LoadPrices(60*5, start, end)
    ReturnJSON(API_OK, plogs)
}
