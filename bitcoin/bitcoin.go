package bitcoin

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/db"
    "ftnox.com/bitcoin/rpc"
)

// GENERAL

// The total blocks returned are at most Coins[coin].TotConf blocks.
// Returns two arrays based on the current blockchain info from rpc.GetBlocks().
// (NOT based on the status of the loaded blocks)
// The status in the returned blocks are either status GOOD, GOOD_CREDITED, or PROCESSING,
// for both 'orphaned' & 'good'.
// It is up to the caller to finish processing.
func LoadAndAssessLastBlocks(coin string) (orphaned, good []*Block) {

    c := Config.GetCoin(coin)

    // Get the last height & hash known in DB.
    // blocks[0] is the latest block.
    blocks := LoadLastBlocks(coin, c.TotConf)

    if len(blocks) == 0 { return nil, nil }

    // TODO: ensure that blocks are actually in the same chain.
    actual := rpc.GetBlocks(coin, blocks[0].Height, blocks[len(blocks)-1].Height)
    if len(actual) != len(blocks) {
        for i, block := range blocks { Warn("blocks@%v\t%v %v", i, block.Height, block.Hash) }
        for i, block := range actual { Warn("actual@%v\t%v %v", i, block.Height, block.Hash) }
        panic(NewError("Expected to fetch %v blocks but only got %v", len(blocks), len(actual)))
    }

    // Iterate over orphaned blocks, working from latest to earliest.
    for i, blk := range blocks {
        if blk.Height != actual[i].Height {
            panic(NewError("Expected actual block height %v but got %v", blk.Height, actual[i].Height))
        }
        actualHash := actual[i].Hash
        if actualHash == blk.Hash {
            good = blocks[i:]
            break
        }
        orphaned = append(orphaned, blk)
    }

    return orphaned, good
}

// Figures out which rpc payments are to known addresses.
func RecognizedPayments(payments []*rpc.RPCPayment) (map[string]*Address, []*rpc.RPCPayment) {
    var recPayments = []*rpc.RPCPayment{}
    var addrsMap = map[string]*Address{}
    addrStrs := []string{}
    for _, payment := range payments {
        addrStrs = append(addrStrs, payment.Address)
    }
    addrs := LoadKnownAddresses(addrStrs)
    for _, addr := range addrs {
        addrsMap[addr.Address] = addr
    }
    for _, payment := range payments {
        if addrsMap[payment.Address] == nil { continue }
        recPayments = append(recPayments, payment)
    }
    return addrsMap, recPayments
}

// SPENDING OUTPUTS

func CheckoutPaymentsToSpend(paymentIds []interface{}, wtxId int64) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        UpdatePaymentsSpent(tx, paymentIds, PAYMENT_SPENT_STATUS_AVAILABLE,
                                            PAYMENT_SPENT_STATUS_CHECKEDOUT, wtxId)
    })
    if err != nil { panic(err) }
}

func MarkPaymentsAsSpent(paymentIds []interface{}, wtxId int64) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        UpdatePaymentsSpent(tx, paymentIds, PAYMENT_SPENT_STATUS_CHECKEDOUT,
                                            PAYMENT_SPENT_STATUS_SPENT, wtxId)
    })
    if err != nil { panic(err) }
}


// CONVERSIONS TO/FROM RPC

func FromRPCPayment(p *rpc.RPCPayment) *Payment {
    return &Payment{
        Coin:           p.Coin,
        TxId:           p.TxId,
        Vout:           p.Vout,
        Blockhash:      p.Blockhash,
        Blockheight:    p.Blockheight,
        Address:        p.Address,
        Amount:         p.Amount,
        ScriptPK:       p.ScriptPK,
        Time:           p.Time,
    }
}

func FromRPCBlock(b *rpc.RPCBlock) *Block {
    return &Block {
        Coin:           b.Coin,
        Height:         b.Height,
        Hash:           b.Hash,
        Time:           b.Time,
    }
}

func ToRPCPayment(p *Payment) *rpc.RPCPayment {
    return &rpc.RPCPayment{
        Coin:           p.Coin,
        TxId:           p.TxId,
        Vout:           p.Vout,
        Blockhash:      p.Blockhash,
        Blockheight:    p.Blockheight,
        Address:        p.Address,
        Amount:         p.Amount,
        ScriptPK:       p.ScriptPK,
        Time:           p.Time,
    }
}

func ToRPCPayments(ps []*Payment) []*rpc.RPCPayment {
    rps := []*rpc.RPCPayment{}
    for _, p := range ps { rps = append(rps, ToRPCPayment(p)) }
    return rps
}

func ToRPCBlock(b *Block) *rpc.RPCBlock {
    return &rpc.RPCBlock {
        Coin:           b.Coin,
        Height:         b.Height,
        Hash:           b.Hash,
        Time:           b.Time,
    }
}
