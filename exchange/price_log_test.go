package exchange

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "testing"
)

func checkPlog(t *testing.T, plog *PriceLog, low, high, op, cl float64, asks, bids uint64) {
    if CompareF64(plog.Low, low, 5) != 0    { t.Fatalf("Expected low of %v, got %v",   low,  plog.Low) }
    if CompareF64(plog.High, high, 5) != 0  { t.Fatalf("Expected high of %v, got %v",  high, plog.High) }
    if CompareF64(plog.Open, op, 5) != 0    { t.Fatalf("Expected open of %v, got %v",  op,   plog.Open) }
    if CompareF64(plog.Close, cl, 5) != 0   { t.Fatalf("Expected close of %v, got %v", cl,   plog.Close) }
    if plog.AskVolume != asks               { t.Fatalf("Expected asks of %v, got %v",  asks, plog.AskVolume) }
    if plog.BidVolume != bids               { t.Fatalf("Expected bids of %v, got %v",  bids, plog.BidVolume) }
}

func TestPriceLog(t *testing.T) {
    logger := &PriceLogger{ Market: RandId(12) }
    logger.Initialize()

    defer func() {
        db.Exec(`
             DELETE FROM exchange_price_log
             WHERE market=?`,
            logger.Market,
        )
    }()

    // Add some entries.
    //             Type, Amount,  Price,  Time
    logger.AddTrade("B",    100,    100,     0) // Minute 0
    logger.AddTrade("B",    100,     99,    10)
    logger.AddTrade("B",    100,    102,    20)

    prices := logger.LoadPrices(60*1, 0, 60*1)
    if len(prices) != 0 { t.Fatalf("Expected 0 prices, got %v", len(prices)) }
    prices = logger.LoadPrices(60*5, 0, 60*5)
    if len(prices) != 0 { t.Fatalf("Expected 0 prices, got %v", len(prices)) }

    logger.AddTrade("B",    100,    105,    60) // Minute 1

    prices = logger.LoadPrices(60*1, 0, 60*1)
    if len(prices) != 1 { t.Fatalf("Expected 1 prices, got %v", len(prices)) }
    checkPlog(t, prices[0], 99, 102, 100, 102, 0, 300)

    logger.AddTrade("B",    100,    104,  60*2) // Minute 2

    prices = logger.LoadPrices(60*1, 0, 60*2)
    if len(prices) != 2 { t.Fatalf("Expected 2 prices, got %v", len(prices)) }
    checkPlog(t, prices[0],  99, 102, 100, 102, 0, 300)
    checkPlog(t, prices[1], 105, 105, 105, 105, 0, 100)
    prices = logger.LoadPrices(60*5, 0, 60*5)
    if len(prices) != 1 { t.Fatalf("Expected 1 prices, got %v", len(prices)) }

    logger.AddTrade("B",    100,    100,  60*6) // Minute 6

    prices = logger.LoadPrices(60*1, 0, 60*6)
    if len(prices) != 3 { t.Fatalf("Expected 3 prices, got %v", len(prices)) }
    checkPlog(t, prices[0],  99, 102, 100, 102, 0, 300)
    checkPlog(t, prices[1], 105, 105, 105, 105, 0, 100)
    checkPlog(t, prices[2], 104, 104, 104, 104, 0, 100)
    prices = logger.LoadPrices(60*5, 0, 60*5)
    if len(prices) != 1 { t.Fatalf("Expected 1 prices, got %v", len(prices)) }
    checkPlog(t, prices[0],  99, 105, 100, 104, 0, 500)
}
