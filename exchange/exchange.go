package exchange

import (
    . "ftnox.com/common"
    "ftnox.com/account"
    "ftnox.com/db"
    "github.com/jaekwon/GoLLRB/llrb"
    "math"
)

const (
    MIN_MEMPOOL = 800
    MAX_MEMPOOL = 1200
)

// All orders go through this channel for processing & cancellations.
// TODO create a channel for each market, thus parallelizing execution.
var ordersCh = make(chan *Order, 200)

// Global, all the markets.
var Markets = map[string]*Market{}
// In order of display
var MarketNames = []string{}

func init() {
    if false{ Info("") }
    initMarkets()
}

func initMarkets() {
    var markets = []*Market{}
    // TODO: refactor out to config
    markets = append(markets, CreateMarket("USD", "BTC"))
    markets = append(markets, CreateMarket("USD", "LTC"))

    for _, market := range markets {
        marketName := market.Coin+"/"+market.BasisCoin
        Markets[marketName] = market
        MarketNames = append(MarketNames, marketName)
    }
}

// Main entry for adding a new order.
// The order gets saved, funds reserved, and added to ordersCh for processing.
// order.Id gets set.
func AddOrder(order *Order) {
    order.Validate()
    SaveAndReserveFundsForOrder(order)
    ordersCh <- order
}

// Main entry for canceling existing (saved) orders.
func CancelOrder(order *Order) {
    order.Validate()
    order.Cancel = true
    ordersCh <- order
}

// This gets called by the daemon.
// NOTE: in the future, this will take a 'market' parameter such that
// each market can run in its own goroutine.
// TODO: refactor & make this a method of Market.
func ProcessNextOrder() (*Order) {
    // Process next order.
    var order = <-ordersCh
    return order.Market().ProcessOrder(order)
}

// A market is where exchanges occur between two currencies.
// The BasisCoin is typically "BTC".
// To disambiguate, a market order will be spelled out as "mOrder"
type Market struct {
    Coin        string
    BasisCoin   string
    Bids        *llrb.LLRB  // min is the best (highest) bid
    Asks        *llrb.LLRB  // min is the best (lowest)  ask
    HasMoreBids bool
    HasMoreAsks bool
    PriceLogger *PriceLogger
}

// Returns the maximum bid price, or 0 if no bids.
func (market *Market) BestBidPrice() (maxBid float64) {
    if market.Bids.Len() > 0 {
        maxBid = market.Bids.Min().(*Order).Price
    }
    return
}

// Returns the minum ask price, or 0 if no asks.
func (market *Market) BestAskPrice() (minAsk float64) {
    if market.Asks.Len() > 0 {
        minAsk = market.Asks.Min().(*Order).Price
    }
    return
}

// Inserts an order into market.Bids/Asks if in range.
// `order` is assumed to be unexecutable.
// Updates market.HasMoreBids/Asks as necessary.
func (market *Market) InsertIfInRange(order *Order) {

    // Insert into market.Bids/Asks
    if order.Type == ORDER_TYPE_BID {
        if market.Bids.Len() < MIN_MEMPOOL ||
           market.Bids.Max().(*Order).Price < order.Price {
            // Insert
            market.Bids.InsertNoReplace(order)
            // If there are too many, prune one.
            if market.Bids.Len() > MAX_MEMPOOL {
                pruned := market.Bids.DeleteMax()
                if pruned != nil { market.HasMoreBids = true }
            }
        } else {
            market.HasMoreBids = true
        }
    } else {
        if market.Asks.Len() < MIN_MEMPOOL ||
           order.Price < market.Asks.Max().(*Order).Price {
            // Insert
            market.Asks.InsertNoReplace(order)
            // If there are too many, prune one.
            if market.Asks.Len() > MAX_MEMPOOL {
                pruned := market.Asks.DeleteMax()
                if pruned != nil { market.HasMoreAsks = true }
            }
        } else {
            market.HasMoreAsks = true
        }
    }
}

// Loads more bids or orders, updating .Asks/.HasMoreAsks or .Bids/.HasMoreBids.
// lastOrderId: We need this for the 'maxId' parameter of loadLimitBids/loadLimitAsks.
//              It must be the same as the result of LastCompletedOrderId()
func (market *Market) LoadMore(orderType string, lastOrderId int64) {
    if orderType == ORDER_TYPE_BID {
        // Maybe we need to load more asks.
        if market.HasMoreAsks && market.Asks.Len() < MIN_MEMPOOL {
            if market.Asks.Len() == 0 { panic("market.HasMoreAsks but no asks in mempool?") }
            moreAsks, hasMoreAsks := LoadLimitAsks(
                market.BasisCoin,
                market.Coin,
                (MAX_MEMPOOL - MIN_MEMPOOL)/2,
                market.Asks.Max().(*Order).Price,
                market.Asks.Max().(*Order).Id,
                lastOrderId,
            )
            for _, ask := range moreAsks {
                market.Asks.InsertNoReplace(llrb.Item(ask))
            }
            market.HasMoreAsks = hasMoreAsks
        }
    } else {
        // Maybe we need to load more bids.
        if market.HasMoreBids && market.Bids.Len() < MIN_MEMPOOL {
            if market.Bids.Len() == 0 { panic("market.HasMoreBids but no bids in mempool?") }
            moreBids, hasMoreBids := LoadLimitBids(
                market.BasisCoin,
                market.Coin,
                (MAX_MEMPOOL - MIN_MEMPOOL)/2,
                market.Bids.Max().(*Order).Price,
                market.Bids.Max().(*Order).Id,
                lastOrderId,
            )
            for _, bid := range moreBids {
                market.Bids.InsertNoReplace(llrb.Item(bid))
            }
            market.HasMoreBids = hasMoreBids
        }
    }
}

// Given a new order, finds the next match, or returns nil
// if not executable.
func (market *Market) NextMatch(order *Order) *Order {
    if order.Type == ORDER_TYPE_BID {
        nxt, ok := market.Asks.Min().(*Order)
        if ok && nxt.Price <= order.Price { return nxt }
    } else if order.Type == ORDER_TYPE_ASK {
        nxt, ok := market.Bids.Min().(*Order)
        if ok && order.Price <= nxt.Price { return nxt }
    } else {
        panic(NewError("Unexpected order type %v", order.Type))
    }
    return nil
}

// Removes order from mempool.
// The caller is responsible for calling market.LoadMore() afterwards.
func (market *Market) DropOrderFromMempool(order *Order) *Order {
    if order.Type == ORDER_TYPE_BID {
        dropped, _ := market.Bids.Delete(order).(*Order)
        return dropped
    } else if order.Type == ORDER_TYPE_ASK {
        dropped, _ := market.Asks.Delete(order).(*Order)
        return dropped
    } else {
        panic(NewError("Unexpected order type %v", order.Type))
    }
}

// Process an order synchronously.
// Returns the most up-to-date version of the order.
func (market *Market) ProcessOrder(order *Order) (*Order) {
    if order.Cancel {
        order := market.ProcessOrderCancellation(order)
        return order
    } else {
        market.ProcessOrderExecution(order)
        return order
    }
}

// Cancels an existing order.
// Returns the most up-to-date version of order.
func (market *Market) ProcessOrderCancellation(order *Order) (*Order) {

    // reload the order, it might have been touched since.
    order = LoadOrder(order.Id)
    switch order.Status {
    case ORDER_STATUS_COMPLETE: return order
    case ORDER_STATUS_CANCELED: return order
    case ORDER_STATUS_PENDING:  break
    default: panic(NewError("Unrecognized order status %v", order.Status))
    }

    // remove it from mempool
    dropped := market.DropOrderFromMempool(order)
    if dropped != nil {
        market.LoadMore(order.Type, order.Id)
    }

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // save the status as canceled
        order.Status = ORDER_STATUS_CANCELED
        UpdateOrder(tx, order)

        // return the reserved funds
        ReleaseReservedFundsForOrder(tx, order)
    })
    if err != nil { panic(err) }
    return order
}

// Executes a new order, which may or may not be a market order.
// The order should have already been saved.
// If the order is executable, it gets executed as well.
// If the order is not executable, or after execution it is
// not complete and no longer executable, it gets added
// into the mempool.
func (market *Market) ProcessOrderExecution(order *Order) {
    if order.Id == 0 { panic("Order hasn't been saved yet") }
    if order.Complete() { panic("New order is already complete.") }

    // Until order is complete, or there are no more matches...
    for {

        match := market.NextMatch(order)
        if match != nil {
            if match.Complete() { panic(NewError("Match %v is already complete.", match.Id)) }

            // Figure out which is bid & ask.
            var bid, ask *Order
            if order.Type == ORDER_TYPE_BID {
                bid, ask = order, match
            } else {
                bid, ask = match, order
            }

            // Figure out how much to trade.
            tradeAmount, tradeBasis, bidBasisFee, askBasisFee := order.ComputeTradeAndFees(match)
            // Info("TradeCoin %v tradeBasis %v", tradeAmount, tradeBasis)

            // Update filled & sanity check before updating the DB.
            bid.Filled += tradeAmount
            bid.BasisFilled += tradeBasis
            bid.BasisFeeFilled += bidBasisFee
            ask.Filled += tradeAmount
            ask.BasisFilled += tradeBasis
            ask.BasisFeeFilled += askBasisFee

            // Update order & match accordingly.
            if order.Complete() { order.Status = ORDER_STATUS_COMPLETE }
            if match.Complete() { match.Status = ORDER_STATUS_COMPLETE }

            // Sanity check
            bid.Validate()
            ask.Validate()
            if askBasisFee > tradeBasis           { panic("askBasisFee exceeded tradeBasis ?!") }
            if !ask.Complete() && !bid.Complete() { panic("Neither ask nor bid was fulfilled after trade.") }

            // Make trade
            trade := &Trade{
                BidUserId:      bid.UserId,
                BidOrderId:     bid.Id,
                BidBasisFee:    bidBasisFee,
                AskUserId:      ask.UserId,
                AskOrderId:     ask.Id,
                AskBasisFee:    askBasisFee,
                Coin:           order.Coin,
                BasisCoin:      order.BasisCoin,
                TradeAmount:    tradeAmount,
                TradeBasis:     tradeBasis,
                Price:          match.Price,
            }

            // Perform transaction.
            // -> update order & match filled & status.
            // -> perform trade of coins between both users.
            // -> return unfilled reserved coins back to the account.WALLET_MAIN wallet.
            err := db.DoBeginSerializable(func(tx *db.ModelTx) {

                UpdateOrder(tx, match)
                UpdateOrder(tx, order)

                // Save trade info.
                SaveTrade(tx, trade)

                // Release remaining reserved funds
                if bid.Complete() { ReleaseReservedFundsForOrder(tx, bid) }
                if ask.Complete() { ReleaseReservedFundsForOrder(tx, ask) }

                // Trade funds & check parity in reserved wallets
                _, err := tx.Exec(`SELECT exchange_do_trade(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
                    bid.Id, bid.UserId, bidBasisFee,
                    ask.Id, ask.UserId, askBasisFee,
                    order.BasisCoin,    tradeBasis,
                    order.Coin,         tradeAmount,
                )
                if err != nil { panic(err) }
            })
            if err != nil { panic(err) }

            // Add trade to price log.
            market.PriceLogger.AddTrade(order.Type, tradeAmount, match.Price, trade.Time)

            // Remove match from mempool if complete.
            if match.Complete() {
                market.DropOrderFromMempool(match)
                market.LoadMore(match.Type, order.Id)
            }

            // Return if we're done with this order.
            if order.Complete() { return }

        } else {

            // There are no more matches for this order,
            // or the order isn't immediatly executable.
            // The order was already saved to DB, but
            // we need to insert it into market.Bids/Asks if in range.
            market.InsertIfInRange(order)
            return
        }
    }
}

func CreateMarket(basisCoin, coin string) *Market {
    marketName := coin+"/"+basisCoin
    numMemPool := (MIN_MEMPOOL+MAX_MEMPOOL)/2

    // First, get the maximum executed (completed) order number.
    // Any pending orders greater than this need to get re-executed.
    // NOTE: this is only an estimate. It's possible that a later order
    // was successfully processed as a new limit order,
    // but it doesn't hurt to re-process them since they have no further side effects.
    lastOrderId := LastCompletedOrderId(basisCoin, coin)

    // Load limit orders.
    bidsSlice, hasMoreBids := LoadLimitBids(basisCoin, coin, numMemPool+1, math.MaxInt64, 0, lastOrderId)
    asksSlice, hasMoreAsks := LoadLimitAsks(basisCoin, coin, numMemPool+1, 0, 0, lastOrderId)
    bids, asks := llrb.New(), llrb.New()
    for _, bid := range bidsSlice { bids.InsertNoReplace(llrb.Item(bid)) }
    for _, ask := range asksSlice { asks.InsertNoReplace(llrb.Item(ask)) }
    market := &Market {
        Coin:           coin,
        BasisCoin:      basisCoin,
        Bids:           bids,
        Asks:           asks,
        HasMoreBids:    hasMoreBids,
        HasMoreAsks:    hasMoreAsks,
        PriceLogger:    &PriceLogger{Market:marketName},
    }
    // TODO: graceful continuing after server restart.
    // currently the PriceLogger is at the BasisInterval scale.
    market.PriceLogger.Initialize()

    // Process pending orders from last app shutdown.
    pendingOrders := LoadPendingOrdersSince(basisCoin, coin, lastOrderId+1)
    if len(pendingOrders) > 0 {
        Info("[%v/%v] Processing %v orders from last shutdown...", coin, basisCoin, len(pendingOrders))
        for _, order := range pendingOrders {
            market.ProcessOrder(order)
        }
        Info("[%v/%v] Done processing.", coin, basisCoin)
    }

    return market
}

// Funds are reserved by moving them to the account.WALLET_RESERVED_ORDER wallet when
// the order is saved to the DB.
// If there aren't enough funds, the order isn't saved, and an error is returned.
// The returned error.Error() is a front-end message.
func SaveAndReserveFundsForOrder(order *Order) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Save the order, get the id
        SaveOrder(tx, order)
        // Reserve the funds
        if order.Type == ORDER_TYPE_BID {
            account.UpdateBalanceByWallet(tx, order.UserId, account.WALLET_MAIN, order.BasisCoin, -int64(order.BasisAmount + order.BasisFee), true)
            account.UpdateBalanceByWallet(tx, order.UserId, account.WALLET_RESERVED_ORDER, order.BasisCoin, int64(order.BasisAmount + order.BasisFee), false)
        } else {
            account.UpdateBalanceByWallet(tx, order.UserId, account.WALLET_MAIN, order.Coin, -int64(order.Amount), true)
            account.UpdateBalanceByWallet(tx, order.UserId, account.WALLET_RESERVED_ORDER, order.Coin, int64(order.Amount), false)
        }
    })
    if err != nil { panic(err) }
}

func ReleaseReservedFundsForOrder(tx *db.ModelTx, order *Order) {
    if order.Status != ORDER_STATUS_COMPLETE &&
       order.Status != ORDER_STATUS_CANCELED { panic(NewError("Cannot release reserved funds for order that isn't complete nor canceled: %v", order.Id)) }

    if order.Type == ORDER_TYPE_BID {
        bid := order
        bidReleaseBasis := (bid.BasisAmount - bid.BasisFilled) + (bid.BasisFee - bid.BasisFeeFilled)
        if bidReleaseBasis > 0 {
            account.UpdateBalanceByWallet(tx, bid.UserId, account.WALLET_RESERVED_ORDER, bid.BasisCoin, -int64(bidReleaseBasis), true)
            account.UpdateBalanceByWallet(tx, bid.UserId, account.WALLET_MAIN, bid.BasisCoin, int64(bidReleaseBasis), false)
        }
    } else if order.Type == ORDER_TYPE_ASK {
        ask := order
        askReleaseAmount := ask.Amount - ask.Filled
        if askReleaseAmount > 0 {
            account.UpdateBalanceByWallet(tx, ask.UserId, account.WALLET_RESERVED_ORDER, ask.Coin, -int64(askReleaseAmount), true)
            account.UpdateBalanceByWallet(tx, ask.UserId, account.WALLET_MAIN, ask.Coin, int64(askReleaseAmount), false)
        }
    } else {
        panic(NewError("Unexpected order type %v", order.Type))
    }
}
