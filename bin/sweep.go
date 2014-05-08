// +build sweep

package main

import (
    . "ftnox.com/common"
    "ftnox.com/bitcoin"
    "ftnox.com/treasury"
    "math"
    "flag"
    "fmt"
    "log"
    "github.com/howeyc/gopass"
)

func main() {

    // Input parameters
    var coin =          flag.String("coin", "", "Coin to sweep")
    var inMPKPubKey =   flag.String("in_mpk", "", "Input MPK pubkey")
    var minInput =      flag.Uint64("min_input", 0, "Minimum permittable input amount")
    var maxInput =      flag.Uint64("max_input", math.MaxInt64, "Maximum permittable input amount")
    var maxTotal =      flag.Uint64("max_total", 0, "Maximum amount of coins to move")
    var maxNumInputs =  flag.Int("max_num_inputs", 10, "Maximum number of inputs per sweep transaction")
    var dryRun =        flag.Bool("dry", true, "Run a dry run")
    // Output parameters
    var outMPKPubKey =  flag.String("out_mpk", "", "Output MPK pubkey")
    var minOutput =     flag.Uint64("min_output", 0, "Minimum permittable output amount")
    var maxOutput =     flag.Uint64("max_output", math.MaxInt64, "Maximum permittable output amount")
    var maxNumOutputs = flag.Int("max_num_outputs", 10, "Maximum number of outputs per sweep transaction")

    flag.Parse()

    fmt.Printf(`Input parameters:
    coin:           %v
    in_mpk:         %v
    min_input:      %v
    max_input:      %v
    max_total:      %v
    max_num_inputs: %v
    dry:            %v

Output parameters:
    out_mpk:        %v
    min_output:     %v
    max_output:     %v
    max_num_outputs %v
    
`, *coin, *inMPKPubKey, *minInput, *maxInput, *maxTotal, *maxNumInputs, *dryRun,
   *outMPKPubKey, *minOutput, *maxOutput, *maxNumOutputs)

    if *coin == "" { log.Panicf("Invalid coin. Wanted: BTC, LTC, etc.") }
    inMPK := bitcoin.LoadMPKByPubKey(*inMPKPubKey)
    if inMPK == nil { log.Panicf("Invalid in_mpk_pubkey") }
    outMPK := bitcoin.LoadMPKByPubKey(*outMPKPubKey)
    if outMPK == nil { log.Panicf("Invalid out_mpk_pubkey") }

    fmt.Println("Enter in_mpk_privkey:")
    inMPKPrivKey := string(gopass.GetPasswd())
    treasury.StorePrivateKeyForMPKPubKey(*inMPKPubKey, inMPKPrivKey)

    // Select a bunch of inputs for sweeping.
    inputs, total, err := treasury.CollectSweepInputs(*coin, inMPK, *minInput, *maxInput, *maxTotal, *maxNumInputs)
    if err != nil { log.Panicf("Error in CollectSweepInputs: %v", err) }
    inputIds := Map(inputs, "Id")

    fmt.Printf("Found %v inputs (total: %v)\n", len(inputs), UI64ToF64(total))
    for _, input := range inputs {
        fmt.Printf("  %v\t%v %v\n", input.Address, input.Coin, UI64ToF64(input.Amount))
    }

    // Create sweep transaction.
    signedTx, _, minerFee, outputs, err := treasury.ComputeSweepTransaction(inputs, outMPK, *minOutput, *maxOutput, *maxNumOutputs, *dryRun)
    if err != nil { log.Panicf("Error in ComputeSweepTransaction: %v", err) }

    fmt.Printf("Computed signed sweep transaction (minerFee: %v)\n", UI64ToF64(minerFee))
    sum := uint64(minerFee)
    for addr, amount := range outputs {
        fmt.Printf("  %v:\t%v\n", addr, UI64ToF64(amount))
        sum += amount
    }
    fmt.Printf("Total: %v\n", UI64ToF64(sum))

    if *dryRun {
        fmt.Println("Dry run complete")
        return
    }

    // DRY RUN ENDS HERE
    // DRY RUN ENDS HERE

    // save WithdrawalTx for bookkeeping
    wthTx := treasury.SaveWithdrawalTx(&treasury.WithdrawalTx{
        Coin:       *coin,
        Type:       treasury.WITHDRAWAL_TX_TYPE_SWEEP,
        Amount:     total,
        MinerFee:   minerFee,
        RawTx:      signedTx,
        TxId:       bitcoin.ComputeTxId(signedTx),
    })

    // checkout those payments.
    bitcoin.CheckoutPaymentsToSpend(inputIds, wthTx.Id)

    // broadcast transaction.
    err = bitcoin.SendRawTransaction(*coin, signedTx)
    if err != nil { panic(err) }

    // update payments as spent.
    bitcoin.MarkPaymentsAsSpent(inputIds, wthTx.Id)

    fmt.Println("Success! Expected TxId: %v (but may change due to malleability)", wthTx.TxId)

}
