/*

This file handles the daemon process for bitcoin deposits.
Each coin is meant to have one daemon goroutine that handles all the deposits.
Code here assumes that it's the only one modifying block & payment status data,
so this should be the only place where block/payments get created and updated.

ALL FUNCTIONS HERE MUST BE IDEMPOTENT.

There are four status states for blocks:
GOOD, PROCESSING, ORPHANED, and GOOD_CREDITED.

State transitions:
    A block starts off as PROCESSING.
    PROCESSING can go to GOOD, ORPHANED.
    GOOD can go to PROCESSING (becoming orphaned) or GOOD_CREDITED (accounts credited).
    ORPHANED can go to PROCESSING (becoming good).
    GOOD_CREDITED can go to PROCESSING (becoming orphaned).

It would also be possible to have two status flags, 'orphaned' and 'credited',
and we might want to transition to that later.

*/

package daemon

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    . "ftnox.com/bitcoin"
    . "ftnox.com/account"
    "ftnox.com/db"
    "github.com/jaekwon/btcjson"
    "ftnox.com/bitcoin/rpc"
    "time"
)

// Continually polls for updates from bitcoin daemon & updates things as necessary.
// Meant to run in a goroutine.
func Sync(coin string) {
    defer Recover("Daemon::Sync("+coin+")")
    Info("[%v] Sync()", coin)
    c := Config.GetCoin(coin)

    for {
        // Compute how many blocks have been orphaned, how many are good.
        orphaned, good := LoadAndAssessLastBlocks(coin)

        // TODO: if there are too many that were orphaned, call OnDisaster()
        // If any blocks have been orphaned, process just the last one and continue.
        if len(orphaned) > 0 {
            orphanBlock(orphaned[0])
            continue
        }

        // We have no blocks to orphan.
        // If any of the good are STATUS_PROCESSING, finish processing as good.
        if len(good) > 0 && good[0].Status == BLOCK_STATUS_PROCESSING {
            // There shouldn't be more than one such good block.
            for _, block := range good[1:] {
                if block.Status != BLOCK_STATUS_GOOD && block.Status != BLOCK_STATUS_GOOD_CREDITED {
                    panic(NewError("Unexpected status for what should be a good block: %v", block.Hash))
                }
            }
            processBlock(good[0])
            continue
        }

        // Get the next new block and process that.
        var nextHeight = uint32(0)
        if len(good) > 0 {
            nextHeight = good[0].Height + 1
        } else {
            nextHeight = rpc.GetCurrentHeight(coin)
        }
        next := rpc.GetBlock(coin, nextHeight)
        if next != nil {
            // First, process the block ReqConf down,
            // those payments should be credited.
            creditDepositsForBlockAtHeight(coin, nextHeight-c.ReqConf+1)
            // Create the new block, or unorphan an existing one.
            block := createOrUnorphanBlock(next)
            // Process the next block which is now in STATUS_PROCESSING.
            processBlock(block)
            continue
        }

        // There are no blocks to process, so sync from the mempool.
        syncMempool(coin, nextHeight)
    }
}

////////////////// CREATING & CREDITING

// We'll check to see whether it already exists.
// If it exists, the status must be of STATUS_ORPHANED.
// If it doesn't exist, a new one will be created.
// The result is a block row that is guaranteed to be in STATE_PROCESSING.
func createOrUnorphanBlock(rpcBlock *rpc.RPCBlock) (*Block) {
    // NOTE: not concurrent given a coin
    // this should be the only place where blocks are created.

    existing := LoadBlock(rpcBlock.Hash)
    if existing == nil {
        block := FromRPCBlock(rpcBlock)
        block.Status = BLOCK_STATUS_PROCESSING
        if block.Time == 0 { block.Time = rpc.TimeForBlock(block.Coin, block.Hash) }
        return SaveBlock(block)
    } else {
        if existing.Status != BLOCK_STATUS_ORPHANED {
            panic(NewError("Invalid state for existing block about to get unorphaned!")) }
        // Change the state to PROCESSING.
        UpdateBlockStatus(existing.Hash, BLOCK_STATUS_ORPHANED, BLOCK_STATUS_PROCESSING)
        return existing
    }
}

// Creates deposits, but does not credit users for payments in this block.
// The block must exist in the DB and be in status PROCESSING.
// The block will go to status GOOD.
func processBlock(block *Block) {
    // SANITY CHECK
    existing := LoadBlock(block.Hash)
    if existing == nil || existing.Status != BLOCK_STATUS_PROCESSING {
        panic(NewError("Something is wrong, processBlock expected an exsting, processing block"))
    }
    // END SANITY CHECK

    rpcPayments := rpc.PaymentsForBlock(block.Coin, block.Hash, true)

    // Filter to recognized payments.
    _, rpcPayments = RecognizedPayments(rpcPayments)

    // Process all the payments for this block.
    for _, rpcPayment := range rpcPayments {
        processPayment(rpcPayment, block)
    }
    // Set status to GOOD
    UpdateBlockStatus(block.Hash, BLOCK_STATUS_PROCESSING, BLOCK_STATUS_GOOD)
}

// Creates payments (or unorphans them) & corresponding deposit rows.
// Does not credit users.
// block: optional, if payment is confirmed by a block.
func processPayment(rpcPayment *rpc.RPCPayment, block *Block) {
    // Create the payment or unorphan one if it already exists.
    payment := createOrUpdatePayment(rpcPayment, block)
    // Create the corresponding deposit if it doesn't already exist.
    maybeCreateDepositForPayment(payment)
}

// Create or updates an existing Payment, associated with the given block.
// When updating, sets the status to GOOD and sets the block* if not already set.
func createOrUpdatePayment(rpcPayment *rpc.RPCPayment, block *Block) (*Payment) {
    // SANITY CHECK
    if block == nil {
        if rpcPayment.Blockhash != "" || rpcPayment.Blockheight != 0 {
            panic(NewError("Something is wrong, a rpcPayment with no block should have no hash or height"))
        }
    } else {
        if rpcPayment.Blockhash != block.Hash || rpcPayment.Blockheight != block.Height {
            panic(NewError("Something is wrong, expected rpcPayment.Blockhash to match block.Hash"))
        }
    }
    // END SANITY CHECK

    payment := FromRPCPayment(rpcPayment)
    _, err := SavePayment(db.GetModelDB(), payment)
    switch db.GetErrorType(err) {
    case db.ERR_DUPLICATE_ENTRY:
        UpdatePayment(db.GetModelDB(), payment)
    default:
        if err != nil { panic(err) }
    }
    return payment
}

// Creates a new deposit row for a payment if it doesn't already exist.
// Does not credit the user.
func maybeCreateDepositForPayment(payment *Payment) {
    if payment.Id == 0 {
        panic(NewError("Something is wrong, payment doesn't have an Id")) }
    LoadOrCreateDepositForPayment(payment)
}

// Credits deposits for a block, if the payment hasn't been credited already.
// The block must be in status GOOD or GOOD_CREDITED.
// The block will go to STATUS_GOOD_CREDITED when done.
func creditDepositsForBlockAtHeight(coin string, height uint32) {
    block := LoadBlockAtHeight(coin, height)
    if block == nil { return }
    if block.Status == BLOCK_STATUS_GOOD_CREDITED { return }

    // SANITY CHECK
    if block.Status != BLOCK_STATUS_GOOD {
        panic(NewError("Block %v isn't in status GOOD. What's up?", height)) }
    // END SANITY CHECK

    payments := LoadPaymentsByBlockhash(block.Hash)
    for _, payment := range payments {
        CreditDepositForPayment(payment)
    }

    // Set status to GOOD_CREDITED
    UpdateBlockStatus(block.Hash, BLOCK_STATUS_GOOD, BLOCK_STATUS_GOOD_CREDITED)
}

////////////////// ORPHANING & UNCREDITING

// Orphans the block, by uncrediting deposits if they were deposited.
// The block must be in status GOOD, GOOD_CREDITED, or PROCESSING.
// The block will go to STATUS_ORPHANED
func orphanBlock(block *Block) {
    // Reload the block from the DB, ensuring that it exists.
    block = LoadBlock(block.Hash)
    if block == nil {
        panic(NewError("Cannot orphan a block that does not exist"))
    }
    if block.Status != BLOCK_STATUS_GOOD &&
       block.Status != BLOCK_STATUS_GOOD_CREDITED &&
       block.Status != BLOCK_STATUS_PROCESSING {
        panic(NewError("Cannot orphan a block that isnt in status GOOD, GOOD_CREDOTED, or PROCESSING"))
    }

    // Set the status to processing.
    if block.Status != BLOCK_STATUS_PROCESSING {
        UpdateBlockStatus(block.Hash, BLOCK_STATUS_GOOD, BLOCK_STATUS_PROCESSING)
    }

    // Orphan all the payments
    payments := LoadPaymentsByBlockhash(block.Hash)
    for _, payment := range payments {
        orphanPayment(payment)
    }

    // Set status to ORPHANED
    UpdateBlockStatus(block.Hash, BLOCK_STATUS_PROCESSING, BLOCK_STATUS_ORPHANED)
}

// If the payment isn't already orphaned, sets it as orphaned.
// If need be, also uncredits the user's account.
func orphanPayment(payment *Payment) {
    // Orphan an existing payment.
    payment.Orphaned = PAYMENT_ORPHANED_STATUS_ORPHANED
    UpdatePayment(db.GetModelDB(), payment)
    // Uncredit deposit if need be.
    balance := UncreditDepositForPayment(payment)
    if balance.Amount < 0 {
        OnNegative(balance)
    }
}


////////////////// SYNCING FROM MEMPOOL

// Cache of unconfirmed transaction hashes
var mempoolTxHashes = NewCMap()

// Looks for updates from the mempool from bitcoin daemon.
// Stops when the height changes.
func syncMempool(coin string, nextHeight uint32) {
    for {
        newHeight := rpc.GetCurrentHeight(coin)
        if nextHeight <= newHeight {
            // TODO: well, not all tx's will have been confirmed.
            mempoolTxHashes.Clear()
            return
        }
        syncMempoolOnce(coin)
        // Wait a bit
        time.Sleep(30 * time.Second)
    }
}

func syncMempoolOnce(coin string) {
    var newTxHashes []string
    txHashes := rpc.UnconfirmedTransactions(coin)
    for _, txHash := range txHashes {
        if !mempoolTxHashes.Has(txHash) {
            mempoolTxHashes.Set(txHash, struct{}{})
            newTxHashes = append(newTxHashes, txHash)
        }
    }

    Info("[%v] Found %v new transactions", coin, len(newTxHashes))

    // Collect all the mempool payments
    rpcPayments := []*rpc.RPCPayment{}
    for _, txHash := range newTxHashes {
        payments, err := rpc.PaymentsForTx(coin, txHash)
        // If the tx is spent & txindex isn't enabled, code is -5.
        if err != nil && err.(*btcjson.Error).Code == -5 { continue }
        if err != nil { panic(err) }
        rpcPayments = append(rpcPayments, payments...)
    }

    Info("[%v] got %v rpcPayments", coin, len(rpcPayments))

    // Filter to recognized payments.
    _, rpcPayments = RecognizedPayments(rpcPayments)

    Info("[%v] filtered to %v recognized rpcPayments", coin, len(rpcPayments))

    // Save payments and create corresponding deposits.
    for _, rpcPayment := range rpcPayments {
        processPayment(rpcPayment, nil)
    }
}

// Callback for when user account balances get uncredited & balance is negative.
func OnNegative(balance *Balance) {
}

// Callback for completely unexpected behavior like the orphaning of way too many blocks.
func OnDisaster(coin string, reason string) {
}
