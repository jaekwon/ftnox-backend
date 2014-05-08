package exchange

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "database/sql"
    "math"
    "time"
    "github.com/jaekwon/GoLLRB/llrb"
)

// Order

type Order struct {
    Id              int64   `json:"id"              db:"id,autoinc"`
    Type            string  `json:"type"            db:"type"`
    UserId          int64   `json:"userId"          db:"user_id"`
    Coin            string  `json:"coin"            db:"coin"`
    Amount          uint64  `json:"amount"          db:"amount"`
    Filled          uint64  `json:"filled"          db:"filled"`
    BasisCoin       string  `json:"basisCoin"       db:"basis_coin"`
    BasisAmount     uint64  `json:"basisAmount"     db:"basis_amount"`
    BasisFilled     uint64  `json:"basisFilled"     db:"basis_filled"`
    BasisFee        uint64  `json:"basisFee"        db:"basis_fee"`
    BasisFeeFilled  uint64  `json:"basisFeeFilled"  db:"basis_fee_filled"`
    BasisFeeRatio   float64 `json:"basisFeeRatio"   db:"basis_fee_ratio"`
    Price           float64 `json:"price"           db:"price"`
    Status          uint32  `json:"status"          db:"status"`
    Cancel          bool    `json:"-"`
    Time            int64   `json:"time"            db:"time"`
    Updated         int64   `json:"updated"         db:"updated"`
}

var OrderModel = db.GetModelInfo(new(Order))

const (
    ORDER_TYPE_BID = "B"
    ORDER_TYPE_ASK = "A"

    ORDER_STATUS_PENDING = 0
    // ORDER_STATUS_INCOMPLETE = 1 (NOT USED, RESERVED)
    ORDER_STATUS_COMPLETE = 2
    ORDER_STATUS_CANCELED = 3
)

func (order *Order) Validate() {

    if 0 < order.Amount && order.Amount < order.Filled                  { panic(NewError("[order: %v] order.Amount < order.Filled", order.Id)) }
    if 0 < order.BasisAmount && order.BasisAmount < order.BasisFilled   { panic(NewError("[order: %v] order.BasisAmount < order.BasisFilled", order.Id)) }

    if order.Type == ORDER_TYPE_BID {
        bid := order
        if bid.BasisAmount == 0                 { panic(NewError("[order: %v] bid.BasisAmount == 0", bid.Id)) }
        if bid.BasisFee    < bid.BasisFeeFilled { panic(NewError("[order: %v] bid.BasisFee    < bid.BasisFeeFilled", bid.Id)) }
        if bid.Complete() &&
           bid.Status != ORDER_STATUS_COMPLETE  { panic(NewError("[order: %v] bid.Status != ORDER_STATUS_COMPLETE", bid.Id)) }
        // bid.Amount may be anything.
    } else if order.Type == ORDER_TYPE_ASK {
        ask := order
        if ask.Amount == 0                      { panic(NewError("[order: %v] ask.Amount == 0", ask.Id)) }
        if ask.Complete() &&
           ask.Status != ORDER_STATUS_COMPLETE  { panic(NewError("[order: %v] ask.Status != ORDER_STATUS_COMPLETE", ask.Id)) }
        // ask.BasisAmount may be anything.
    } else {
        panic(NewError("Invalid order type %v for order: %v", order.Type, order.Id))
    }
}

func (order *Order) MarketName() string {
    return order.Coin+"/"+order.BasisCoin
}

func (order *Order) Market() *Market {
    return Markets[order.MarketName()]
}

func (order *Order) SortBidAsk(match *Order) (bid *Order, ask *Order) {
    if order.Type == ORDER_TYPE_BID {
        if match.Type != ORDER_TYPE_ASK { panic(NewError("Cannot match bid & bid")) }
        return order, match
    } else {
        if match.Type != ORDER_TYPE_BID { panic(NewError("Cannot match ask & ask")) }
        return match, order
    }
}

func (order *Order) ComputeTradeAndFees(match *Order) (tradeAmount, tradeBasis, bidBasisFee, askBasisFee uint64) {
    tradeAmount, tradeBasis = order.ComputeTrade(match)
    bid, ask := order.SortBidAsk(match)
    // compute basis_coin fee for bid
    bidBasisFee = uint64(float64(bid.BasisFeeRatio) * float64(tradeBasis) + 0.5)
    if bidBasisFee > (bid.BasisFee - bid.BasisFeeFilled) {
        bidBasisFee = bid.BasisFee - bid.BasisFeeFilled
    }
    // compute basis_coin fee for ask
    askBasisFee = uint64(float64(ask.BasisFeeRatio) * float64(tradeBasis) + 0.5)
    return tradeAmount, tradeBasis, bidBasisFee, askBasisFee
}

func (order *Order) ComputeTrade(match *Order) (tradeAmount, tradeBasis uint64) {
    if order.Amount == 0 && order.BasisAmount == 0 { panic(NewError("Order has no limitation")) }
    if match.Amount == 0 && match.BasisAmount == 0 { panic(NewError("Match has no limitation")) }

    /* The logic can actually compute the trade for arbitrary sets of restrictions,
     but for now we limit it to cases where bids have a BasisAmount limit (no Amount limit),
     and asks have an Amount limit (no BasisAmount limit).
    This is to keep the coin reservation system simple.
    Comment out the below section to enable mixed limitations. */
    /*
    bid, ask := order.SortBidAsk(match)
    if bid.Amount > 0  || bid.BasisAmount == 0 { panic(NewError("Bids must have BasisAmount limit, and no Amount limit.")) }
    if ask.Amount == 0 || ask.BasisAmount > 0  { panic(NewError("Asks must have Amount limit, and no BasisAmount limit.")) }
    */
    /* End section */

    var price = match.Price

    var orderAmountRemaining uint64
    if order.Amount > 0 {       orderAmountRemaining = order.Amount - order.Filled
    } else {                    orderAmountRemaining = math.MaxUint64 }
    var orderBasisRemaining uint64
    if order.BasisAmount > 0 {  orderBasisRemaining = order.BasisAmount - order.BasisFilled
    } else {                    orderBasisRemaining = math.MaxUint64 }
    var matchAmountRemaining uint64
    if match.Amount > 0 {       matchAmountRemaining = match.Amount - match.Filled
    } else {                    matchAmountRemaining = math.MaxUint64 }
    var matchBasisRemaining uint64
    if match.BasisAmount > 0 {  matchBasisRemaining = match.BasisAmount - match.BasisFilled
    } else {                    matchBasisRemaining = math.MaxUint64 }

    var amountRemaining = MinUint64(orderAmountRemaining, matchAmountRemaining)
    var basisRemaining =  MinUint64(orderBasisRemaining,  matchBasisRemaining)

    if amountRemaining != math.MaxUint64 {
        basisRemaining2 := uint64(float64(amountRemaining) * float64(price) + 0.5)
        if basisRemaining == math.MaxUint64 {
            return amountRemaining, basisRemaining2
        } else if basisRemaining >= basisRemaining2 {
            return amountRemaining, basisRemaining2
        } else {
            amountRemaining2 := uint64(float64(basisRemaining) / float64(price) + 0.5)
            if amountRemaining2 > amountRemaining {
                // Not sure if this happens or not. Probably possible. :P
                return amountRemaining, basisRemaining
            } else {
                return amountRemaining2, basisRemaining
            }
        }
    } else {
        amountRemaining2 := uint64(float64(basisRemaining) / float64(price) + 0.5)
        return amountRemaining2, basisRemaining
    }
}

func (order *Order) Complete() bool {
    if order.Amount > 0 && order.Amount == order.Filled {
        return true
    }
    if order.BasisAmount > 0 && order.BasisAmount == order.BasisFilled {
        return true
    }
    return false
}

// The least item is the one closest to the last price.
// TODO: account for float64, we shouldn't be comparing by equality. 
func (order *Order) Less(than llrb.Item) bool {
    other, ok := than.(*Order)
    if !ok { panic("Cannot compare order with something else ") }
    if order.Id == other.Id {
        return false
        //panic(NewError("Cannot compare identical items of id %v", order.Id))
    }
    if order.Type != other.Type { panic("Cannot compare bid & ask") }
    if order.Type == ORDER_TYPE_BID {
        if order.Price > other.Price { return true }
        if order.Price == other.Price { return order.Id < other.Id }
        return false
    } else {
        if order.Price < other.Price { return true }
        if order.Price == other.Price { return order.Id < other.Id }
        return false
    }
}

func LoadOrder(id int64) (*Order) {
    var order Order
    err := db.QueryRow(
        `SELECT `+OrderModel.FieldsSimple+`
         FROM exchange_order
         WHERE id=?`, id,
    ).Scan(&order)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &order
    default:
        panic(err)
    }
}

func SaveOrder(tx *db.ModelTx, order *Order) (*Order) {
    if order.Time == 0 { order.Time = time.Now().Unix() }
    err := tx.QueryRow(
        `INSERT INTO exchange_order (`+OrderModel.FieldsInsert+`)
         VALUES (`+OrderModel.Placeholders+`)
         RETURNING id`,
        order,
    ).Scan(&order.Id)
    if err != nil { panic(err) }
    return order
}

func UpdateOrder(tx *db.ModelTx, order *Order) {
    order.Updated = time.Now().Unix()
    _, err := tx.Exec(
        `UPDATE exchange_order
         SET status=?, filled=?, basis_filled=?, updated=?
         WHERE id=?`,
        order.Status, order.Filled, order.BasisFilled, order.Updated, order.Id,
    )
    if err != nil { panic(err) }
}

// Gets the last executed order id, or 0 if none.
func LastCompletedOrderId(basisCoin string, coin string) int64 {
    var lastCompletedOrderId int64
    err := db.QueryRow(
        `SELECT id FROM exchange_order
         WHERE basis_coin=? AND coin=? AND status=3
         ORDER BY id DESC limit 1`,
        basisCoin, coin,
    ).Scan(&lastCompletedOrderId)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return 0
    case nil:
        return lastCompletedOrderId
    default:
        panic(err)
    }
}

// Loads bid orders less than or equal to maxPrice,
//  but id greater than minId if price is maxPrice.
// This way you can either set maxPrice to MaxUint64 (or MaxInt64 for postgres)
//  or set maxPrice & minId to the last bid loaded to load more.
// Also, order ids must be less than or equal to maxId.
// (Orders with ids greater than maxId may not be limit orders & may need to get reprocessed.)
func LoadLimitBids(basisCoin string, coin string, limit int, maxPrice float64, minId int64, maxId int64) (bids []*Order, hasMore bool) {
    rows, err := db.QueryAll(Order{},
        `SELECT `+OrderModel.FieldsSimple+`
         FROM exchange_order
         WHERE basis_coin=? AND coin=? AND type='B' AND status=0 AND (price, id) < (?, ?) AND id<=?
         ORDER BY price DESC, id ASC LIMIT ?`,
        basisCoin, coin, maxPrice, int64(-1) * minId, maxId, limit+1,
    )
    if err != nil { panic(err) }
    bids = rows.([]*Order)
    if len(bids) == limit+1 {
        return bids[:len(bids)-1], true
    } else {
        return bids, false
    }
}

// See comment for LoadLimitBids.
// To load the best asks, set minPrice to 0.
func LoadLimitAsks(basisCoin string, coin string, limit int, minPrice float64, minId int64, maxId int64) (asks []*Order, hasMore bool) {
    rows, err := db.QueryAll(Order{},
        `SELECT `+OrderModel.FieldsSimple+`
         FROM exchange_order
         WHERE basis_coin=? AND coin=? AND type='A' AND status=0 AND (price, id) > (?, ?) AND id<=?
         ORDER BY price ASC, id ASC LIMIT ?`,
        basisCoin, coin, minPrice, minId, maxId, limit+1,
    )
    if err != nil { panic(err) }
    asks = rows.([]*Order)
    if len(asks) == limit+1 {
        return asks[:len(asks)-1], true
    } else {
        return asks, false
    }
}

// This is for loading pending orders upon app init,
//  for orders that were saved but didn't go through the processor.
// NOTE: don't try to add a limit // paginate here, unless you understand why this notice is here.
func LoadPendingOrdersSince(basisCoin string, coin string, startOrderId int64) []*Order {
    rows, err := db.QueryAll(Order{},
        `SELECT `+OrderModel.FieldsSimple+`
         FROM exchange_order
         WHERE basis_coin=? AND coin=? AND status=0 AND id>=?
         ORDER BY id ASC`,
        basisCoin, coin, startOrderId,
    )
    if err != nil { panic(err) }
    return rows.([]*Order)
}

func LoadPendingOrdersByUser(userId int64, basisCoin string, coin string) (orders []*Order) {
    rows, err := db.QueryAll(Order{},
        `SELECT `+OrderModel.FieldsSimple+`
         FROM exchange_order
         WHERE status=0 AND user_id=? AND basis_coin=? AND coin=?
         ORDER BY price ASC`,
        userId, basisCoin, coin,
    )
    if err != nil { panic(err) }
    return rows.([]*Order)
}

// Trade

type Trade struct {
    Id          int64   `json:"id"              db:"id,autoinc"`
    BidUserId   int64   `json:"bidUserId"       db:"bid_user_id"`
    BidOrderId  int64   `json:"bidOrderId"      db:"bid_order_id"`
    BidBasisFee uint64  `json:"bidBasisFee"     db:"bid_basis_fee"`
    AskUserId   int64   `json:"askUserId"       db:"ask_user_id"`
    AskOrderId  int64   `json:"askOrderId"      db:"ask_order_id"`
    AskBasisFee uint64  `json:"askBasisFee"     db:"ask_basis_fee"`
    Coin        string  `json:"coin"            db:"coin"`
    BasisCoin   string  `json:"basisCoin"       db:"basis_coin"`
    TradeAmount uint64  `json:"tradeAmount"     db:"trade_amount"`
    TradeBasis  uint64  `json:"tradeBasis"      db:"trade_basis"`
    Price       float64 `json:"price"           db:"price"`
    Time        int64   `json:"time"            db:"time"`
}

var TradeModel = db.GetModelInfo(new(Trade))

func SaveTrade(tx *db.ModelTx, trade *Trade) (*Trade) {
    if trade.Time == 0 { trade.Time = time.Now().Unix() }
    err := tx.QueryRow(
        `INSERT INTO exchange_trade (`+TradeModel.FieldsInsert+`)
         VALUES (`+TradeModel.Placeholders+`)
         RETURNING id`,
        trade,
    ).Scan(&trade.Id)
    if err != nil { panic(err) }
    return trade
}

func LoadTradesByUser(userId int64, limit uint) []*Trade {
    rows, err := db.QueryAll(Trade{},
        `SELECT `+TradeModel.FieldsSimple+`
         FROM exchange_trade
         WHERE (bid_user_id=? OR ask_user_id=?)
         ORDER BY time DESC LIMIT ?`,
        userId, userId, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Trade)
}

// Price Log

type PriceLog struct {
    Id          int64       `json:"-"           db:"id,autoinc"`
    Market      string      `json:"-"           db:"market"`
    Low         float64     `json:"l"           db:"low"`
    High        float64     `json:"h"           db:"high"`
    Open        float64     `json:"o"           db:"open"`
    Close       float64     `json:"c"           db:"close"`
    Interval    int64       `json:"-"           db:"interval"`
    AskVolume   uint64      `json:"a"           db:"ask_volume"`
    BidVolume   uint64      `json:"b"           db:"bid_volume"`
    Time        int64       `json:"t"           db:"time"`
    Timestamp   time.Time   `json:"-"           db:"timestamp"`
}

var PriceLogModel = db.GetModelInfo(new(PriceLog))

func SaveOrUpdatePriceLog(tx *db.ModelTx, plog *PriceLog) {
    var exists int
    err := tx.QueryRow(
        `SELECT 1 FROM exchange_price_log
         WHERE market=? AND interval=? AND time=?`,
        plog.Market, plog.Interval, plog.Time,
    ).Scan(&exists)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        SavePriceLog(tx, plog)
    case nil:
        UpdatePriceLog(tx, plog)
    default:
        panic(err)
    }
}

func SavePriceLog(tx *db.ModelTx, plog *PriceLog) (*PriceLog) {
    err := tx.QueryRow(
        `INSERT INTO exchange_price_log (`+PriceLogModel.FieldsInsert+`)
         VALUES (`+PriceLogModel.Placeholders+`)
         RETURNING id`,
        plog,
    ).Scan(&plog.Id)
    if err != nil { panic(err) }
    return plog
}

func UpdatePriceLog(tx *db.ModelTx, plog *PriceLog) {
    _, err := tx.Exec(
        `UPDATE exchange_price_log
         SET low=?, high=?, close=?, ask_volume=?, bid_volume=?
         WHERE market=? AND interval=? AND time=?`,
        plog.Low, plog.High, plog.Close, plog.AskVolume, plog.BidVolume,
        plog.Market, plog.Interval, plog.Time,
    )
    if err != nil { panic(err) }
}

func LoadPriceLogs(market string, interval int64, startTime int64, endTime int64) []*PriceLog {
    rows, err := db.QueryAll(PriceLog{},
        `SELECT `+PriceLogModel.FieldsSimple+`
         FROM exchange_price_log
         WHERE market=? AND interval=? AND ?<=time AND time<?
         ORDER BY time ASC`,
        market, interval, startTime, endTime,
    )
    if err != nil { panic(err) }
    return rows.([]*PriceLog)
}

func LoadLastPriceLogs(market string, interval int64, limit int) []*PriceLog {
    rows, err := db.QueryAll(PriceLog{},
        `SELECT `+PriceLogModel.FieldsSimple+`
         FROM exchange_price_log
         WHERE market=? AND interval=?
         ORDER BY time DESC LIMIT ?`,
        market, interval, limit,
    )
    if err != nil { panic(err) }
    plogs := rows.([]*PriceLog)
    for i:=0; i<len(plogs)/2; i++ { plogs[i] = plogs[len(plogs)-1-i] } // time ASC order
    return plogs
}
