package bitcoin

import (
    //. "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/bitcoin/rpc"
    "time"
)

const CURRENT_HEIGHT_CACHE_SEC = 10

// Convenience functions.
// They could be member functions of the Coin struct,
// but we want the Coin struct to be shared.

func CurrentHeight(name string) uint32 {
    coin := Config.GetCoin(name)
    // Caches currentHeight for 10 seconds.
    if coin.CurrentHeightTime == 0 ||
       coin.CurrentHeightTime+CURRENT_HEIGHT_CACHE_SEC < time.Now().Unix() {
        currentHeight := rpc.GetCurrentHeight(coin.Name)
        coin.CurrentHeight = currentHeight
        coin.CurrentHeightTime = time.Now().Unix()
    }
    return coin.CurrentHeight;
}

// This is the height needed for confirmed deposit.
// e.g.
// current height is 3,
// Coins["BTC"].ReqConf is 3,
// Then ReqHeight("BTC") is 1.
func ReqHeight(name string) uint32 {
    currentHeight := CurrentHeight(name)
    return currentHeight - Config.GetCoin(name).ReqConf + 1
}

func MinWithdrawAmount(name string) uint64 {
    return Config.GetCoin(name).MinerFee * 2 // TODO: adjust.
}

func MinerFee(name string) uint64 {
    return Config.GetCoin(name).MinerFee
}
