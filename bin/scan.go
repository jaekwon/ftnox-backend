// +build scan

package main

import (
    . "ftnox.com/common"
    "ftnox.com/bitcoin"
    "github.com/jaekwon/btcjson"
)

func receive(ch chan *btcjson.Vout) {
    for {
        vout := <-ch
        //Info(vout.ScriptPubKey.Type, " ", vout.ScriptPubKey.Asm, " ", vout.ScriptPubKey.Hex, " ", vout.ScriptPubKey.ReqSig, " ", len(vout.ScriptPubKey.Addresses))
        type_ := vout.ScriptPubKey.Type
        len_  := len(vout.ScriptPubKey.Hex)
        if !(
            type_ == "pubkeyhash" && len_ == 50 ||
            type_ == "pubkey" && len_ == 70 ||
            type_ == "pubkey" && len_ == 134 ||
            type_ == "scripthash" && len_ == 46) {
            Info(type_, len_)
        }
    }
}

func main() {
    ch := make(chan *btcjson.Vout)
    go receive(ch)
    err := bitcoin.ScanTxFromHeight("BTC", 176000, ch)
    Warn(err)
}
