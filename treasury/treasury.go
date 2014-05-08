package treasury

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/db"
    "ftnox.com/account"
    "ftnox.com/bitcoin/rpc"
    "ftnox.com/bitcoin"
    "ftnox.com/alert"
    "fmt"
    "time"
)

var masterPrivKeys = NewCMap()
var hotMPK *bitcoin.MPK

const MAX_BASE_FEES = 10

func init() {
    hotMPK = account.GetHotMPK()
}

// The purpose of this is to seed the privKey into memory without
// keeping it on file. It's a bit more secure that way.
// It gets called by treasury/handlers.go/StorePrivateKeyHandler.
func StorePrivateKeyForMPKPubKey(pubKey string, privKey string) {
    masterPrivKeys.Set(pubKey, privKey)
}

func hasHotPrivKey() bool {
    pubKey := hotMPK.PubKey
    privKey, _ := masterPrivKeys.Get(pubKey).(string)
    return privKey != ""
}

func ComputePrivateKeyForAddress(addr *bitcoin.Address) string {
    mpk := bitcoin.LoadMPK(addr.MPKId)
    if mpk == nil { panic(NewError("Failed to find mpk %v", addr.MPKId)) }
    masterPrivKey, _ := masterPrivKeys.Get(mpk.PubKey).(string)
    if masterPrivKey == "" { panic(NewError("Failed to find privkey for mpk %v", mpk.PubKey)) }
    privKey := bitcoin.ComputePrivateKey(masterPrivKey, mpk.Chain, addr.ChainPath, addr.ChainIdx)
    derivedAddr := bitcoin.ComputeAddressForPrivKey(addr.Coin, privKey)
    if addr.Address != derivedAddr {
        panic(NewError("Computed invalid private key for address. addr: %v, derivedAddr: %v", addr.Address, derivedAddr))
    }
    return privKey
}

// Each coin should only have 1 processor.
// This allows for more granular transactions.
// NOTE: consider select ... for update when selecting the payments to use.
// CONTRACT: call this in a goroutine, it won't stop until a fatal error.
func Process(coin string) {
    defer Recover("Treasury::Process("+coin+")")

    for {

        // if privkey hasn't been seeded yet, then wait.
        if !hasHotPrivKey() {
            Warn("Please seed the master privKey for %v", account.GetHotMPK().PubKey)
            time.Sleep(60 * time.Second)
            continue
        }

        // Process sweep transactions
        // TODO: automatically figure out sweeps.
        // TODO: for now, get sweeps from admin.

        // Process user initiated withdrawals
        processed, err := ProcessUserWithdrawals(coin)
        if err != nil {
            // Notify admin of the error, will retry manually.
            alert.Alert(fmt.Sprintf("Withdrawals for %v stalled: %v", coin, err.Error()))
            continue
        } else if !processed {
            Info("Sleeping, no withdrawals to process")
            time.Sleep(30 * time.Second)
            continue
        }

    }
}

// Returns false if no withdrawals are available to process.
func ProcessUserWithdrawals(coin string) (bool, error) {

    // Checkout withdrawals
    // TODO: Gather multiple small withdrawals.
    wths := account.CheckoutWithdrawals(coin, 1)
    if len(wths) == 0 {
        return false, nil
    }
    wthIds := Map(wths, "Id")
    amounts := map[string]uint64{}
    amountSum := uint64(0)
    for _, wth := range wths {
        if wth.Amount <= 0 { panic(NewError("Invalid send amount %v", wth.Amount)) }
        amounts[wth.ToAddress] += uint64(wth.Amount)
        amountSum += uint64(wth.Amount)
    }

    // figure out which payments to use.
    signedTx, payments, minerFees, chgAddress, err := ComputeWithdrawalTransaction(coin, amounts)
    if err != nil {
        account.StallWithdrawals(wthIds)
        return false, err
    }
    paymentIds := Map(payments, "Id")

    // save withdrawal info for bookkeeping.
    wthTx := SaveWithdrawalTx(&WithdrawalTx{
        Coin:       coin,
        Type:       WITHDRAWAL_TX_TYPE_WITHDRAWAL,
        Amount:     amountSum,
        MinerFee:   minerFees,
        ChgAddress: chgAddress,
        RawTx:      signedTx,
        TxId:       bitcoin.ComputeTxId(signedTx),
    })

    // checkout those payments.
    bitcoin.CheckoutPaymentsToSpend(paymentIds, wthTx.Id)

    // TODO: the Tx should go out to our partners who sign them for us.
    // TODO: receive the signed Tx.

    // deduct change amount from system user's "change" wallet.
    // this creates a negative balance, which will revert to zero
    // when the change is received.
    if chgAddress != "" {
        changeAmount := amounts[chgAddress]
        err := db.DoBeginSerializable(func(tx *db.ModelTx) {
            account.UpdateBalanceByWallet(tx, 0, account.WALLET_CHANGE, coin, -int64(changeAmount), false)
        })
        if err != nil { panic(err) }
    }

    // broadcast transaction.
    rpc.SendRawTransaction(coin, signedTx)

    // update payments as spent.
    bitcoin.MarkPaymentsAsSpent(paymentIds, wthTx.Id)

    // update withdrawals as complete.
    account.CompleteWithdrawals(wths, wthTx.Id)

    return true, nil
}

func maxMinerFeeForCoin(coin string) uint64 {
    return uint64(bitcoin.MinerFee(coin)) * MAX_BASE_FEES
}

func collectPrivateKeys(coin string, payments []*bitcoin.Payment, privKeys map[string]string) {
    for _, payment := range payments {
        if privKeys[payment.Address] != "" {
            continue
        } else {
            addr := bitcoin.LoadAddress(payment.Address)
            privKey := ComputePrivateKeyForAddress(addr)
            privKeyWIF := bitcoin.ComputeWIF(coin, privKey, true)
            privKeys[payment.Address] = privKeyWIF
            continue
        }
    }
}

// Given inputs and outputs, and given that the maximum fee has already been deducted
// such that sum(inputs) - maximum_fee = sum(outputs),
// Adjust outputs such that leftover fees go into changeAddress.
// To prevent dust, if outputs[changeAddress] ends up being dust,
// it is omitted.
// 'privKeys' will be updated include all the private keys for output addresses.
func adjustMinerFee(coin string, inputs []*bitcoin.Payment, outputs map[string]uint64, changeAddress string, privKeys map[string]string) (uint64, error) {
    c := Config.GetCoin(coin)
    maxMinerFee := maxMinerFeeForCoin(coin)
    inputSum := sumInputs(inputs)
    outputSum := sumOutputs(outputs)
    if inputSum < outputSum + maxMinerFee { return 0, NewError("Inputs didn't exceed outputs + maximum miner fee") }
    collectPrivateKeys(coin, inputs, privKeys)
    // Make RPC call to sign.
    s := rpc.CreateSignedRawTransaction(coin, bitcoin.ToRPCPayments(inputs), outputs, privKeys)
    // Figure out how many base fees we actually need.
    numKBytes := len(s)/2 / 1000
    requiredBaseFees := numKBytes + 1
    if requiredBaseFees > MAX_BASE_FEES { return 0, NewError("Whoa, too many base fees required: %v (max %v)", requiredBaseFees, MAX_BASE_FEES) }
    requiredFee := uint64(requiredBaseFees) * uint64(c.MinerFee)
    // Add remainder back to changeAddress
    if maxMinerFee > requiredFee {
        outputs[changeAddress] += (maxMinerFee - requiredFee)
    }
    // Remove dust
    if outputs[changeAddress] < c.MinerFee {
        delete(outputs, changeAddress)
    }
    return requiredFee, nil
}

func sumInputs(payments []*bitcoin.Payment) uint64 {
    sumAmount := uint64(0)
    for _, payment := range payments { sumAmount += payment.Amount }
    return sumAmount
}

func sumOutputs(outputs map[string]uint64) uint64 {
    sumAmount := uint64(0)
    for _, output := range outputs { sumAmount += output }
    return sumAmount
}

func createNewChangeAddress(coin string) string {
    chainPath := fmt.Sprintf("%v", bitcoin.CHAINPATH_PREFIX_CHANGE)
    address := bitcoin.CreateNewAddress(coin, 0, "change", hotMPK, chainPath)
    return address.Address
}

// dry: uses a different chainpath meant to throw away.
func createNewSweepAddress(coin string, mpk *bitcoin.MPK, dry bool) string {
    var chainPath string
    var wallet string
    if dry {
        chainPath = fmt.Sprintf("%v", bitcoin.CHAINPATH_PREFIX_SWEEP_DRY)
        wallet = account.WALLET_SWEEP_DRY
    } else {
        chainPath = fmt.Sprintf("%v", bitcoin.CHAINPATH_PREFIX_SWEEP)
        wallet = account.WALLET_SWEEP
    }
    address := bitcoin.CreateNewAddress(coin, 0, wallet, mpk, chainPath)
    return address.Address
}

// Finds payments & constructs transaction to satisfy the given output amounts.
// This function could fail, in which case we'll call it again later.
// This means that this function should be largely side-effect free.
// However, note that 'outputs' may be modified to account for
// fees & change addresses.
func ComputeWithdrawalTransaction(coin string, outputs map[string]uint64) (string, []*bitcoin.Payment, uint64, string, error) {
    returnErr := func(err error) (string, []*bitcoin.Payment, uint64, string, error) { return "", nil, 0, "", err }

    reqHeight := bitcoin.ReqHeight(coin)
    payments  := []*bitcoin.Payment{}
    changeAddress := createNewChangeAddress(coin)

    sumAmount := int64(0)
    for _, amount := range outputs {
        sumAmount += int64(amount)
    }
    // Add base fees to sumAmount. Be generous, we'll adjust later.
    sumAmount += int64(bitcoin.MinerFee(coin)) * MAX_BASE_FEES
    sumAmountCopy := sumAmount
    // Then load payments greater than remainder
    for sumAmountCopy > 0 {
        // We shouldn't use too many inputs.
        if len(payments) > len(outputs) * 2 {
            return returnErr(NewError("[%v] Too many inputs required for %v", coin, sumAmount))
        }
        // Try to do it with one input
        payment := bitcoin.LoadSmallestSpendablePaymentGreaterThan(hotMPK.Id, coin, uint64(sumAmountCopy), reqHeight, payments)
        if payment == nil {
            // Try to fill it as much as possible
            payment = bitcoin.LoadLargestSpendablePaymentLessThan(hotMPK.Id, coin, uint64(sumAmountCopy), reqHeight, payments)
        }
        if payment == nil {
            return returnErr(NewError("[%v] Unable to gather enough inputs for %v", coin, sumAmount))
        }
        sumAmountCopy -= int64(payment.Amount)
        payments = append(payments, payment)
    }
    // If we need to create a change address, do so.
    if sumAmountCopy != 0 {
        outputs[changeAddress] = uint64(-1 * sumAmountCopy)
    }
    // Adjust miner fees & collect private keys
    privKeys := map[string]string{}
    minerFee, err := adjustMinerFee(coin, payments, outputs, changeAddress, privKeys)
    if err != nil { return returnErr(err) }
    // Sign transaction
    s := rpc.CreateSignedRawTransaction(coin, bitcoin.ToRPCPayments(payments), outputs, privKeys)
    return s, payments, minerFee, changeAddress, nil
}

// Given constraints of `minOutput`, `maxOutput` amounts,
// divide `total` into at most `maxNumOutputs` elements.
// If possible spread the outputs out evenly.
// TODO: spread it out expoentially instead of linearly?
func computeSweepOutputs(total, minOutput, maxOutput uint64, maxNumOutputs int) ([]uint64, bool) {
    if maxOutput * uint64(maxNumOutputs) < total {
        // Impossible.
        return nil, false
    } else if total < minOutput {
        // We don't have enough input to satisfy minInput.
        // Might as well just run with it.
        return []uint64{total}, true
    } else {
        // Perfect.
        outputAmounts := []uint64{}
        numOutputs := int(float64(total) / float64((minOutput + maxOutput) / uint64(2)) + 0.5)
        if numOutputs > maxNumOutputs {
            numOutputs = maxNumOutputs
        } else if numOutputs == 0 {
            numOutputs = 1
        }
        avgOutput := float64(total) / float64(numOutputs)
        maxDeviation := -1.0
        if avgOutput > float64((minOutput + maxOutput)/2) {
            maxDeviation = float64(maxOutput) - avgOutput
        } else {
            maxDeviation = avgOutput - float64(minOutput)
        }
        // Get output estimates
        for i:=0; i<numOutputs/2; i++ {
            // 2 -> ±1/1 (of maxDeviation)
            // 3 -> ±2/2
            // 4 -> ±3/3, ±1/3
            // 5 -> ±4/4, ±2/4
            // 6 -> ±5/5, ±3/5, ±1/5
            deviation := float64((numOutputs-1)-(i*2))/float64(numOutputs-1) * maxDeviation
            outputAmounts = append(outputAmounts, uint64(avgOutput + deviation + 0.5))
            outputAmounts = append(outputAmounts, uint64(avgOutput - deviation + 0.5))
        }
        if numOutputs%2 == 1 {
            outputAmounts = append(outputAmounts, uint64(avgOutput + 0.5))
        }
        // The first element of outputAmounts is the greatest.
        // Adjust as necessary & remove fees.
        sumOutputs := uint64(0)
        for _, output := range outputAmounts { sumOutputs += output }
        diff := total - sumOutputs
        if diff > 0 {
            outputAmounts[len(outputAmounts)-1] += diff
        } else if diff < 0 {
            outputAmounts[0] += diff
        }
        return outputAmounts, true
    }
}

// Collects up to maxTotal coins of tx outs between minInput and maxInput coins in size.
// - inMPK: Set inMPK to hotMPK to move funds offline from the hot wallet.
// - minInput, maxInput: Range of permittable input transaction sizes.
// - maxTotal: Maximum amount of coins to collect.
// NOTE: should be deterministic, because of dryRun options.
func CollectSweepInputs(coin string, inMPK *bitcoin.MPK, minInput, maxInput, maxTotal uint64, maxNumInputs int) ([]*bitcoin.Payment, uint64, error) {
    reqHeight := bitcoin.ReqHeight(coin)

    if minInput < bitcoin.MinerFee(coin) {
        return nil, 0, NewError("minInput must be at least as large as the miner fee for %v: %v", coin, maxMinerFeeForCoin(coin))
    }

    // Gather oldest spendable inputs in range until maxTotal.
    candidates := bitcoin.LoadOldestSpendablePaymentsBetween(inMPK.Id, coin, minInput, maxInput, maxNumInputs, reqHeight)
    inputs := []*bitcoin.Payment{}
    total := uint64(0)
    for _, payment := range candidates {
        if total+payment.Amount > maxTotal { continue }
        total += payment.Amount
        inputs = append(inputs, payment)
    }

    // total must meet some threshold for sweep to be worth it.
    if total < maxMinerFeeForCoin(coin) {
        return nil, 0, NewError("Could not gather enough inputs. Needed at least %v, only got %v", maxMinerFeeForCoin(coin), total)
    }

    return inputs, total, nil
}

// Sweeps inputs into new address(s) generated for mpk.
// - outMPK: Set outMPK to offline wallet MPK to move funds offline
// - minOutput, maxOutput: Range of permittable output transaction sizes.
//      The outputs will be spread out evenly between minOutput & maxOutput linearly
//       though if maxOutput is greater than the sum of all the inputs minus fees,
//       there will only be one output.
//      This means you could set maxOutput to MaxInt64 and you'll be guaranteed to have one output.
// - dry: dry run. The output addresses will be throwaway addresses.
func ComputeSweepTransaction(inputs []*bitcoin.Payment, outMPK *bitcoin.MPK, minOutput, maxOutput uint64, maxNumOutputs int, dry bool) (string, []*bitcoin.Payment, uint64, map[string]uint64, error) {
    returnErr := func(err error) (string, []*bitcoin.Payment, uint64, map[string]uint64, error) { return "", nil, 0, nil, err }

    var coin = inputs[0].Coin
    for _, payment := range inputs {
        if payment.Coin != coin { return returnErr(NewError("Expected all sweep inputs to be for coin %v", coin)) }
    }

    // compute total inputs
    total := uint64(0)
    for _, payment := range inputs { total += payment.Amount }

    // compute sweep output spread
    outputAmounts, ok := computeSweepOutputs(total, minOutput, maxOutput, maxNumOutputs)
    if !ok { return returnErr(NewError("Could not satisfy output requirements")) }

    // Remove maxMinerFees from output initially.
    // We'll readjust later
    outputAmounts[0] -= maxMinerFeeForCoin(coin)

    // Adjust miner fees & collect private keys
    outputs := map[string]uint64{}
    changeAddress := "" // not really a change address, but remaining fees get added back here.
    for _, amount := range outputAmounts {
        address := createNewSweepAddress(coin, outMPK, dry)
        if changeAddress == "" { changeAddress = address }
        outputs[address] = amount
    }
    privKeys := map[string]string{}
    minerFee, err := adjustMinerFee(coin, inputs, outputs, changeAddress, privKeys)
    if err != nil { return returnErr(err) }

    // Sign transaction
    s := rpc.CreateSignedRawTransaction(coin, bitcoin.ToRPCPayments(inputs), outputs, privKeys)
    return s, inputs, minerFee, outputs, nil
}


// MODELS

// This represents an outbound transaction.
// the Payment.wtx_id gets set to this Id for record keeping,
// so we know why the input was spent.
type WithdrawalTx struct {
    Id          int64  `json:"id"           db:"id,autoinc"`
    Type        string `json:"type"         db:"type"`
    Coin        string `json:"coin"         db:"coin"`
    FromMPKId   int64  `json:"fromMPKId"    db:"from_mpk_id,null"`
    ToMPKId     int64  `json:"toMPKId"      db:"to_mpk_id,null"`
    Amount      uint64 `json:"amount"       db:"amount"`
    MinerFee    uint64 `json:"minerFee"     db:"miner_fee"`
    ChgAddress  string `json:"chgAddress"   db:"chg_address"`
    RawTx       string `json:"rawTx"        db:"raw_tx"`
    TxId        string `json:"txId"         db:"tx_id"`
    Time        int64  `json:"time"         db:"time"`
}

var WithdrawalTxModel = db.GetModelInfo(new(WithdrawalTx))

const (
    WITHDRAWAL_TX_TYPE_WITHDRAWAL = "W" // user withdrawal
    WITHDRAWAL_TX_TYPE_SWEEP = "S"      // e.g. from hot to cold wallet, etc.
)

func SaveWithdrawalTx(wth *WithdrawalTx) (*WithdrawalTx) {
    if wth.Time == 0 { wth.Time = time.Now().Unix() }
    err := db.QueryRow(
        `INSERT INTO withdrawal_tx (`+WithdrawalTxModel.FieldsInsert+`)
         VALUES (`+WithdrawalTxModel.Placeholders+`)
         RETURNING id`,
        wth,
    ).Scan(&wth.Id)
    if err != nil { panic(err) }
    return wth
}
