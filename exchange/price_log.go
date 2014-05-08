/*
NOTE: If the server restarts, it'll lose PriceLogger.current, a minute's worth of data.
TODO: Delete all but the last X rows from smaller intervals.
*/

package exchange

import (
    // . "ftnox.com/common"
    "ftnox.com/db"
    "time"
    "math"
)

var Intervals = []int64{
    60,         // minute (basis)
    60*5,       // 5 minutes
    60*60,      // hour
}
var BasisInterval = Intervals[0]
var LongInterval  = Intervals[len(Intervals)-1]

type PriceLogger struct {
    Market  string
    entries []*PriceLog // of basis time interval
    current *PriceLog
}

// Initialize by loading basis entries from the DB
func (logger *PriceLogger) Initialize() {
    logger.entries = LoadLastPriceLogs(logger.Market, Intervals[0], int(LongInterval/BasisInterval))
}

// Assuming that all the PriceLog entries for the time range given with (t, interval)
// are in logger.entries, compute the finalized PriceLog for this interval.
// Returns nil if no trades were in range of (t, interval)
func (logger *PriceLogger) computeForInterval(t int64, interval int64) *PriceLog {
    startTime := t / interval * interval
    endTime   := startTime + interval
    inRange   := []*PriceLog{}
    for _, plog := range logger.entries {
        if plog.Time < startTime || endTime < (plog.Time + plog.Interval) { continue }
        inRange = append(inRange, plog)
    }
    if len(inRange) == 0 { return nil }
    newPlog := PriceLog{
        Market:     logger.Market,
        Interval:   interval,
        Time:       startTime,
        Timestamp:  time.Unix(startTime, 0),
        High:       float64(0),
        Low:        math.MaxUint64,
        Open:       inRange[0].Open,
        Close:      inRange[len(inRange)-1].Close,
    }
    for _, plog := range inRange {
        if plog.Low < newPlog.Low { newPlog.Low = plog.Low }
        if newPlog.High < plog.High { newPlog.High = plog.High }
        newPlog.AskVolume += plog.AskVolume
        newPlog.BidVolume += plog.BidVolume
    }
    return &newPlog
}

// plog: The basis PriceLog entry to save
// nextTime: The next PriceLog entry will have this time.
//  * This is how we know whether to finalize parent blogs.
func (logger *PriceLogger) addPriceLog(plog *PriceLog, nextTime int64) {
    if plog.Interval != BasisInterval   { panic("addPriceLog() expects smallest interval") }
    if plog.Time % plog.Interval != 0   { panic("plog.Time % Interval should be zero") }
    if plog.Market != logger.Market     { panic("plog.Market wasn't logger.market") }

    logger.entries = append(logger.entries, plog)

    toSave := []*PriceLog{plog}
    for i, interval := range Intervals {
        if i == 0 { continue }
        // If this interval window has closed...
        //if (plog.Time / interval) < (nextTime / interval) {
        toSave = append(toSave, logger.computeForInterval(plog.Time, interval))
        //}
    }

    // TODO: this doesn't have to be serializable
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        for _, plog := range toSave {
            SaveOrUpdatePriceLog(tx, plog)
        }
    })
    if err != nil { panic(err) }

}

// Main function for adding datapoints.
func (logger *PriceLogger) AddTrade(orderType string, amount uint64, price float64, t int64) {
    t = t / BasisInterval * BasisInterval
    var bidVolume, askVolume uint64
    if orderType == ORDER_TYPE_BID {
        bidVolume = amount
    } else if orderType == ORDER_TYPE_ASK {
        askVolume = amount
    }
    // Add & finalize basis & more intervals as necessary.
    if logger.current != nil && logger.current.Time < t {
        diff := t - logger.current.Time
        if diff % BasisInterval != 0 { panic("diff % BasisInterval != 0") }
        logger.addPriceLog(logger.current, t)
        logger.current = nil
    }
    if logger.current == nil {
        logger.current = &PriceLog{
            Market:     logger.Market,
            Low:        price,
            High:       price,
            Open:       price,
            Interval:   BasisInterval,
            Time:       t,
            Timestamp:  time.Unix(t, 0),
        }
    }
    current := logger.current
    if price < current.Low { current.Low = price }
    if current.High < price { current.High = price }
    current.AskVolume += askVolume
    current.BidVolume += bidVolume
    current.Close = price
}

// TODO: Caching? I suppose that depends on usage.
func (logger *PriceLogger) LoadPrices(interval int64, start int64, end int64) []*PriceLog {
    start = (start / interval) * interval
    plogs := LoadPriceLogs(logger.Market, interval, start, end)
    return plogs
}

// Returns 0 if none.
func (logger *PriceLogger) LastPrice() float64 {
    if logger.current != nil {
        return logger.current.Close
    } else if len(logger.entries) > 0 {
        return logger.entries[len(logger.entries)-1].Close
    }
    return 0
}
