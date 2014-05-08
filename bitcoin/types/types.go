package types

// Represents a supported coin (e.g. BTC, LTC) of the exchange.
// Loaded from ~/.ftnox.com/config.json
type Coin struct {
    Name        string
    Symbol      string
    Type        string

    // Trade
    MinTrade    uint64

    // Crypto
    ConfSec     uint32
    RPCUser     string
    RPCPass     string
    RPCHost     string
    TotConf     uint32
    ReqConf     uint32
    SyncStart   uint32
    AddrPrefix  byte
    WIFPrefix   byte
    MinerFee    uint64

    // cache currentHeight
    CurrentHeightTime   int64
    CurrentHeight       uint32
}

const (
    COIN_TYPE_CRYPTO = "C"
    COIN_TYPE_FIAT =   "F"
)
