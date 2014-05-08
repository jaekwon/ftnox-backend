package rpc

type RPCPayment struct {
    Coin        string
    TxId        string
    Vout        uint32
    Blockhash   string
    Blockheight uint32
    Address     string
    Amount      uint64
    ScriptPK    string
    Time        int64
}

type RPCBlock struct {
    Coin        string
    Height      uint32
    Hash        string
    Time        int64
}
