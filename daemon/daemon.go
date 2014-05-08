package daemon

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    bitcoin "ftnox.com/bitcoin/types"
    "ftnox.com/treasury"
    "ftnox.com/exchange"
)

// Cache of unconfirmed transaction hashes
// TODO: set expiry on items, or use redis.
var unconfirmedTxHashes = NewCMap()

func init() {
    Info("DAEMON STARTED")
    for _, coin := range Config.Coins {
        if coin.Type == bitcoin.COIN_TYPE_CRYPTO {
            go Sync(coin.Name)
            go treasury.Process(coin.Name)
        }
    }
    go ProcessOrders()
}

func ProcessOrders() {
    defer Recover("Daemon::ProcessOrders")
    for {
        order := exchange.ProcessNextOrder()
        if false {Debug("[%v] Processed order %v", order.MarketName(), order.Id)}
    }
}
