package account

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/db"
    "ftnox.com/auth"
    "ftnox.com/bitcoin"
    "fmt"
)

// Master public key for generating account deposit addresses
var hotMPK *bitcoin.MPK

func init() {
    hotMPK = bitcoin.SaveMPKIfNotExists(&bitcoin.MPK{
        PubKey: Config.HotMPKPubKey,
        Chain:  Config.HotMPKChain,
    })
}

func GetHotMPK() *bitcoin.MPK {
    return hotMPK
}

// BALANCE

func LoadBalances(userId int64, wallet string) map[string]int64 {
    var balMap = map[string]int64{}
    balances := LoadBalancesByWallet(userId, wallet)
    for _, balance := range balances {
        balMap[balance.Coin] = balance.Amount
    }
    for _, coin := range Config.Coins {
        if _, ok := balMap[coin.Name]; !ok {
            balMap[coin.Name] = int64(0)
        }
    }
    return balMap
}

func LoadAllDepositAddresses(userId int64, wallet string) map[string]string {
    var addrMap = map[string]string{}
    addrs := bitcoin.LoadAddressesByWallet(userId, wallet)
    for _, addr := range addrs {
        addrMap[addr.Coin] = addr.Address
    }
    return addrMap
}

func LoadOrCreateDepositAddress(userId int64, wallet string, coin string) string {
    user := auth.LoadUser(userId)
    chainPath := fmt.Sprintf("%v/%v", bitcoin.CHAINPATH_PREFIX_DEPOSIT, user.ChainIdx)
    address := bitcoin.LoadLastAddressByWallet(userId, wallet, coin)
    if address == nil {
        address = bitcoin.CreateNewAddress(coin, userId, wallet, hotMPK, chainPath)
    }
    return address.Address
}

// WITHDRAWAL

func AddWithdrawal(userId int64, toAddr string, coin string, amount uint64) (*Withdrawal, error) {
    wth := &Withdrawal{
        UserId:         userId,
        Wallet:         WALLET_MAIN,
        Coin:           coin,
        ToAddress:      toAddr,
        Amount:         amount,
        Status:         WITHDRAWAL_STATUS_PENDING,
    }

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // save withdrawal
        SaveWithdrawal(tx, wth)
        // adjust balance.
        UpdateBalanceByWallet(tx, userId, WALLET_MAIN, coin, -int64(amount), true)
        UpdateBalanceByWallet(tx, userId, WALLET_RESERVED_WITHDRAWAL, coin, int64(amount), false)
    })
    return wth, err
}

func CheckoutWithdrawals(coin string, limit uint) (wths []*Withdrawal) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        wths = LoadWithdrawalsByStatus(tx, coin, WITHDRAWAL_STATUS_PENDING, limit)
        wthIds := Map(wths, "Id")

        UpdateWithdrawals(tx, wthIds, WITHDRAWAL_STATUS_PENDING,
                                      WITHDRAWAL_STATUS_CHECKEDOUT, 0)
    })
    if err != nil { panic(err) }
    return
}

func CompleteWithdrawals(wths []*Withdrawal, wtxId int64) {
    wthIds := Map(wths, "Id")
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // update status
        UpdateWithdrawals(tx, wthIds, WITHDRAWAL_STATUS_CHECKEDOUT,
                                      WITHDRAWAL_STATUS_COMPLETE, wtxId)
        // adjust balance
        for _, wth := range wths {
            UpdateBalanceByWallet(tx, wth.UserId, WALLET_RESERVED_WITHDRAWAL, wth.Coin, -int64(wth.Amount), true)
        }
    })
    if err != nil { panic(err) }
}

func StallWithdrawals(wthIds []interface{}) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // update status
        UpdateWithdrawals(tx, wthIds, WITHDRAWAL_STATUS_CHECKEDOUT,
                                      WITHDRAWAL_STATUS_STALLED, 0)
    })
    if err != nil { panic(err) }
}

func ResumeWithdrawals(wthIds []interface{}) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // update status
        UpdateWithdrawals(tx, wthIds, WITHDRAWAL_STATUS_STALLED,
                                      WITHDRAWAL_STATUS_PENDING, 0)
    })
    if err != nil { panic(err) }
}

func CancelWithdrawal(wth *Withdrawal) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // update status
        UpdateWithdrawals(tx, []interface{}{wth.Id}, WITHDRAWAL_STATUS_PENDING,
                                                   WITHDRAWAL_STATUS_CANCELED, 0)
        // adjust balance
        UpdateBalanceByWallet(tx, wth.UserId, WALLET_RESERVED_WITHDRAWAL, wth.Coin, -int64(wth.Amount), true)
        UpdateBalanceByWallet(tx, wth.UserId, WALLET_MAIN, wth.Coin, int64(wth.Amount), false)
    })
    if err != nil { panic(err) }
}

// TRANSFER (not used anymore/yet)

func AddTransfer(fromUserId int64, fromWallet string, toUserId int64, toWallet string, coin string, amount uint64) error {
    // Create new transfer item that moves amount.
    trans := &Transfer{
        UserId:         fromUserId,
        Wallet:         fromWallet,
        User2Id:        toUserId,
        Wallet2:        toWallet,
        Coin:           coin,
        Amount:         amount,
        Fee:            uint64(0),
    }

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Adjust balance
        UpdateBalanceByWallet(tx, trans.UserId, trans.Wallet, trans.Coin, -int64(trans.Amount), true)
        UpdateBalanceByWallet(tx, trans.User2Id, trans.Wallet2, trans.Coin, int64(trans.Amount), false)
        // Save transfer
        SaveTransfer(tx, trans)
    })
    return err
}

// DEPOSIT

// This just creates a new row in the accounts_deposits table.
// It doesn't actually credit the account, etc.
// Must be idempotent.
func LoadOrCreateDepositForPayment(payment *bitcoin.Payment) (*Deposit) {

    if payment.Id == 0 { panic(NewError("Cannot add deposit for unsaved payment")) }

    addr := bitcoin.LoadAddress(payment.Address)
    if addr == nil { panic(NewError("Expected address for payment to deposit")) }

    deposit := &Deposit{
        Type:       DEPOSIT_TYPE_CRYPTO,
        UserId:     addr.UserId,
        Wallet:     addr.Wallet,
        Coin:       addr.Coin,
        Amount:     payment.Amount,
        PaymentId:  payment.Id,
        Status:     DEPOSIT_STATUS_PENDING,
    }
    _, err := SaveDeposit(db.GetModelDB(), deposit)
    switch db.GetErrorType(err) {
    case db.ERR_DUPLICATE_ENTRY:
        return LoadDepositForPayment(db.GetModelDB(), payment.Id)
    default:
        if err != nil { panic(err) }
    }

    return deposit
}

// Credit the user's account for the given payment.
// If the deposit is already credited, do nothing.
// Must be idempotent.
func CreditDepositForPayment(payment *bitcoin.Payment) {

    // SANITY CHECK
    paymentId := payment.Id
    // Reload the payment.
    payment = bitcoin.LoadPaymentByTxId(payment.TxId, payment.Vout)
    // Ensure that it isn't orphaned.
    if payment.Orphaned != bitcoin.PAYMENT_ORPHANED_STATUS_GOOD {
        panic(NewError("Cannot credit deposit for an orphaned payment")) }
    // Are you paranoid enough?
    if paymentId != payment.Id {
        panic(NewError("payment.Id didn't match")) }
    // END SANITY CHECK

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Load the corresponding deposit.
        deposit := LoadDepositForPayment(tx, payment.Id)
        // If the deposit isn't pending, do nothing.
        if deposit.Status != DEPOSIT_STATUS_PENDING { return }
        // Credit the account.
        UpdateBalanceByWallet(tx, deposit.UserId, deposit.Wallet, deposit.Coin, int64(deposit.Amount), false)
        UpdateDepositSetStatus(tx, deposit.Id, DEPOSIT_STATUS_CREDITED)
    })
    if err != nil { panic(err) }
}

// Uncredit the user's account for the given payment.
// If the deposit isn't credited, do nothing.
// Returns true if the resulting balance is negative.
// Must be idempotent.
func UncreditDepositForPayment(payment *bitcoin.Payment) (balance *Balance) {

    // SANITY CHECK
    paymentId := payment.Id
    // Reload the payment.
    payment = bitcoin.LoadPaymentByTxId(payment.TxId, payment.Vout)
    // Ensure that it is orphaned.
    if payment.Orphaned != bitcoin.PAYMENT_ORPHANED_STATUS_ORPHANED {
        panic(NewError("Cannot uncredit deposit for a payment that isn't in STATUS_ORPHANED")) }
    // Are you paranoid enough?
    if paymentId != payment.Id {
        panic(NewError("payment.Id didn't match")) }
    // END SANITY CHECK

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Load the corresponding deposit.
        deposit := LoadDepositForPayment(tx, payment.Id)
        // If the deposit isn't credited, do nothing.
        if deposit.Status != DEPOSIT_STATUS_CREDITED { return }
        // Uncredit the account.
        balance = UpdateBalanceByWallet(tx, deposit.UserId, deposit.Wallet, deposit.Coin, -int64(deposit.Amount), false)
        UpdateDepositSetStatus(tx, deposit.Id, DEPOSIT_STATUS_PENDING)
    })
    if err != nil { panic(err) }
    return
}

// This just creates a new row in the accounts_deposits table.
// It doesn't actually credit the account, etc.
// Must be idempotent.
func CreateDeposit(deposit *Deposit) (*Deposit) {

    // SANITY CHECK
    if deposit.Id != 0 { panic(NewError("Expected a new deposit but got a saved one")) }
    if deposit.Type != DEPOSIT_TYPE_FIAT { panic(NewError("Expected a fiat (bank) deposit")) }
    if deposit.PaymentId != 0 { panic(NewError("Fiat (bank) deposit cannot have a payment")) } // TODO move to validator.
    if deposit.Status != DEPOSIT_STATUS_PENDING { panic(NewError("Expected a pending deposit")) }
    // END SANITY CHECK

    _, err := SaveDeposit(db.GetModelDB(), deposit)
    if err != nil { panic(err) }

    return deposit
}

// Credit the user's account for the given bank deposit.
// If the deposit is already credited, do nothing.
// Must be idempotent.
func CreditDeposit(deposit *Deposit) {

    // SANITY CHECK
    if deposit.Id == 0 { panic(NewError("Expected a saved deposit but got a new one")) }
    // END SANITY CHECK

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Load the corresponding deposit.
        deposit := LoadDeposit(tx, deposit.Id)
        // If the deposit isn't pending, do nothing.
        if deposit.Status != DEPOSIT_STATUS_PENDING { return }
        // Credit the account.
        UpdateBalanceByWallet(tx, deposit.UserId, deposit.Wallet, deposit.Coin, int64(deposit.Amount), false)
        UpdateDepositSetStatus(tx, deposit.Id, DEPOSIT_STATUS_CREDITED)
    })
    if err != nil { panic(err) }
}

// Uncredit the user's account for the given payment.
// If the deposit isn't credited, do nothing.
// Returns true if the resulting balance is negative.
// Must be idempotent.
func UncreditDeposit(deposit *Deposit) (balance *Balance) {

    // SANITY CHECK
    if deposit.Id == 0 { panic(NewError("Expected a saved deposit but got a new one")) }
    // END SANITY CHECK

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Load the corresponding deposit.
        deposit := LoadDeposit(tx, deposit.Id)
        // If the deposit isn't credited, do nothing.
        if deposit.Status != DEPOSIT_STATUS_CREDITED { return }
        // Uncredit the account.
        balance = UpdateBalanceByWallet(tx, deposit.UserId, deposit.Wallet, deposit.Coin, -int64(deposit.Amount), false)
        UpdateDepositSetStatus(tx, deposit.Id, DEPOSIT_STATUS_PENDING)
    })
    if err != nil { panic(err) }
    return
}
