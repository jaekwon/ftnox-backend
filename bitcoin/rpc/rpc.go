package rpc

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "github.com/jaekwon/btcjson"
    //"github.com/davecgh/go-spew/spew"
)

func SendRPCSafe(coin string, message string, args ...interface{}) (interface{}, error) {
    Debug("[%v] RPC SendRPC(%v, %v)", coin, message, args)
    c := Config.GetCoin(coin)
    msg, err := btcjson.CreateMessage(message, args...)
    if err != nil { return nil, err }
    reply, err := btcjson.RpcCommand(
        c.RPCUser, c.RPCPass, c.RPCHost, msg)
    if err != nil { return nil, err }
    if reply.Error != nil { return nil, reply.Error }
    return reply.Result, err
}

func SendRPC(coin string, message string, args ...interface{}) (interface{}) {
    res, err := SendRPCSafe(coin, message, args...)
    if err != nil { panic(err) }
    return res
}

// TODO: cache answer?
func GetCurrentHeight(coin string) (uint32) {
    info_i := SendRPC(coin, "getinfo")
    info := info_i.(btcjson.InfoResult)
    return uint32(info.Blocks)
}

// Does not fill in block.Time
// TODO: ensure that blocks are in the same chain.
func GetBlocks(coin string, startHeight uint32, endHeight uint32) ([]*RPCBlock) {
    var blocks []*RPCBlock
    if startHeight < endHeight {
        for height:=startHeight; height<=endHeight; height++ {
            hash := HashForHeight(coin, height)
            blocks = append(blocks, &RPCBlock{coin, height, hash, 0})
        }
        return blocks
    } else {
        for height:=startHeight; height>=endHeight; height-- {
            hash := HashForHeight(coin, height)
            blocks = append(blocks, &RPCBlock{coin, height, hash, 0})
        }
        return blocks
    }
}

// Does not fill in block.Time
func GetBlock(coin string, height uint32) (*RPCBlock) {
    currentHeight := GetCurrentHeight(coin)
    if height > currentHeight { return nil }
    hash := HashForHeight(coin, height)
    return &RPCBlock{coin, height, hash, 0}
}

func HashForHeight(coin string, height uint32) (string) {
    hash_i := SendRPC(coin, "getblockhash", int(height))
    return hash_i.(string)
}

func PaymentsForTx(coin string, hash string) ([]*RPCPayment, error) {
    var payments []*RPCPayment

    tx_i, err := SendRPCSafe(coin, "getrawtransaction", hash, 1)
    if err != nil { return nil, err }
    tx := tx_i.(btcjson.TxRawResult)

    // Don't process any coinbase outputs for now.
    // In the future we should handle these specially, as
    // they require more confirmations.
    if tx.Vin[0].IsCoinBase() { return nil, nil }

    for voutN, vout := range tx.Vout {
        if vout.ScriptPubKey.Type == "pubkey" || vout.ScriptPubKey.Type == "pubkeyhash" {
            amount, err := btcjson.JSONToAmount(vout.Value)
            if err != nil { panic(err) }
            payment := RPCPayment{
                Coin:        coin,
                TxId:        tx.Txid,
                Vout:        uint32(voutN),
                Blockhash:   "", //block.Hash,
                Blockheight: 0,  //uint32(block.Height),
                Address:     vout.ScriptPubKey.Addresses[0],
                Amount:      uint64(amount),
                ScriptPK:    vout.ScriptPubKey.Hex,
                Time:        int64(tx.Time),
            }
            payments = append(payments, &payment)
        }
    }

    return payments, nil
}

// TODO: requires -txindex
func ScanTxFromHeight(coin string, height uint32, ch chan *btcjson.Vout) {
    for ;;height++ {
        Info("%v", height)
        hash := HashForHeight(coin, height)
        block := blockResultForHash(coin, hash)
        for _, txhash := range block.Tx {
            tx_i := SendRPC(coin, "getrawtransaction", txhash, 1)
            tx := tx_i.(btcjson.TxRawResult)
            for _, vout := range tx.Vout {
                ch <- &vout
            }
        }
    }
}

func blockResultForHash(coin string, hash string) (*btcjson.BlockResult) {
    block_i := SendRPC(coin, "getblock", hash)
    block := block_i.(btcjson.BlockResult)
    return &block
}

func TimeForBlock(coin string, hash string) (int64) {
    block := blockResultForHash(coin, hash)
    return int64(block.Time)
}

func PaymentsForBlock(coin string, hash string, skipSpentTx bool) ([]*RPCPayment) {
    var payments []*RPCPayment

    block := blockResultForHash(coin, hash)
    for _, txhash := range block.Tx {
        txPayments, err := PaymentsForTx(coin, txhash)
        // If the tx is spent & txindex isn't enabled, code is -5.
        if btcjsonErr, ok := err.(*btcjson.Error); ok && skipSpentTx && btcjsonErr.Code == -5 { continue }
        if err != nil { panic(err) }
        for _, payment := range txPayments {
            payment.Blockhash = block.Hash
            payment.Blockheight = uint32(block.Height)
        }
        payments = append(payments, txPayments...)
    }

    return payments
}

func UnconfirmedTransactions(coin string) ([]string) {
    txs_i := SendRPC(coin, "getrawmempool")
    txs := txs_i.([]string)
    return txs
}

func CreateSignedRawTransaction(coin string, payments []*RPCPayment, outputs map[string]uint64, privKeys map[string]string) (string) {
    inputs := []interface{}{}
    for _, payment := range payments {
        inputs = append(inputs, map[string]interface{}{
            "txid": payment.TxId,
            "vout": payment.Vout,
        })
    }
    outputsF := map[string]float64{}
    for addr, amount := range outputs { outputsF[addr] = I64ToF64(int64(amount)) }
    rawTx := SendRPC(coin, "createrawtransaction", inputs, outputsF)

    inputs = []interface{}{}
    for _, payment := range payments {
        inputs = append(inputs, map[string]interface{}{
            "txid": payment.TxId,
            "vout": payment.Vout,
            "scriptPubKey": payment.ScriptPK,
        })
    }
    privKeysArray := []string{}
    for _, privKey := range privKeys { privKeysArray = append(privKeysArray, privKey) }
    signedTx_i := SendRPC(coin, "signrawtransaction", rawTx, inputs, privKeysArray)
    signedTx := signedTx_i.(btcjson.SignRawTransactionResult)
    if !signedTx.Complete { panic(NewError("Failed to sign transaction")) }

    //Debug("[%v] Created signed transaction %v", coin, signedTx.Hex)

    return signedTx.Hex
}

func SendRawTransaction(coin string, rawTx string) {
    Info("[%v] Sending raw tx, rawTx: %v", coin, rawTx)
    SendRPC(coin, "sendrawtransaction", rawTx)
}
