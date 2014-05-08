package account

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "database/sql"
    "time"
)

const (
    WALLET_MAIN =                   "main"
    WALLET_RESERVED_ORDER =         "reserved_o"
    WALLET_RESERVED_WITHDRAWAL =    "reserved_w"
    WALLET_SWEEP =                  "sweep"
    WALLET_SWEEP_DRY =              "sweep_dry"
    WALLET_CHANGE =                 "change"
)

// BALANCE

type Balance struct {
    UserId  int64   `json:"userId"      db:"user_id"`
    Wallet  string  `json:"wallet"      db:"wallet"`
    Coin    string  `json:"coin"        db:"coin"`
    Amount  int64   `json:"amount"      db:"amount"`
}

var BalanceModel = db.GetModelInfo(new(Balance))

func SaveBalance(c db.MConn, balance *Balance) (*Balance) {
    _, err := c.Exec(
        `INSERT INTO account_balance (`+BalanceModel.FieldsInsert+`)
         VALUES (`+BalanceModel.Placeholders+`)`,
        balance,
    )
    if err != nil { panic(err) }
    return balance
}

// Adds or subtracts an amount to a user's wallet.
// nonnegative: panics with INSUFFICIENT_FUNDS_ERROR if resulting balance is negative.
// Returns the new balance
func UpdateBalanceByWallet(tx *db.ModelTx, userId int64, wallet string, coin string, diff int64, nonnegative bool) *Balance {
    var balance Balance

    // Get existing balance.
    err := tx.QueryRow(
        `SELECT `+BalanceModel.FieldsSimple+`
         FROM account_balance WHERE
         user_id=? AND wallet=? AND coin=?`,
        userId, wallet, coin,
    ).Scan(&balance)

    // Ensure nonnegative
    if nonnegative && balance.Amount+diff < 0 {
        panic(INSUFFICIENT_FUNDS_ERROR)
    }

    // First time balance?
    if err == sql.ErrNoRows {
        // Create new balance
        balance := Balance{UserId:userId, Wallet:wallet, Coin:coin, Amount:diff}
        SaveBalance(tx, &balance)
        return &balance
    }

    // Update balance
    balance.Amount += diff
    _, err = tx.Exec(
        `UPDATE account_balance
         SET amount=?
         WHERE user_id=? AND wallet=? AND coin=?`,
        balance.Amount, userId, wallet, coin,
    )
    if err != nil { panic(err) }
    return &balance
}

func LoadBalancesByWallet(userId int64, wallet string) []*Balance {
    rows, err := db.QueryAll(Balance{},
        `SELECT `+BalanceModel.FieldsSimple+`
         FROM account_balance
         WHERE user_id=? AND wallet=?`,
        userId, wallet,
    )
    if err != nil { panic(err) }
    return rows.([]*Balance)
}

// DEPOSIT

type Deposit struct {
    Id          int64   `json:"id"              db:"id,autoinc"`
    Type        string  `json:"type"            db:"type"`
    UserId      int64   `json:"userId"          db:"user_id"`
    Wallet      string  `json:"wallet"          db:"wallet"`
    Coin        string  `json:"coin"            db:"coin"`
    Amount      uint64  `json:"amount"          db:"amount"`
    PaymentId   int64   `json:"paymentid"       db:"payment_id,null"`
    Status      int32   `json:"status"          db:"status"`
    Time        int64   `json:"time"            db:"time"`
    Updated     int64   `json:"updated"         db:"updated"`
}

var DepositModel = db.GetModelInfo(new(Deposit))

const (
    DEPOSIT_TYPE_CRYPTO = "C"
    DEPOSIT_TYPE_FIAT =   "F"

    DEPOSIT_STATUS_PENDING = 0
    DEPOSIT_STATUS_CREDITED = 1
)

// Might throw an error if the deposit already exists.
func SaveDeposit(c db.MConn, dep *Deposit) (*Deposit, error) {
    if dep.Time == 0 { dep.Time = time.Now().Unix() }
    // Add to DB
    err := c.QueryRow(
        `INSERT INTO account_deposit (`+DepositModel.FieldsInsert+`)
         VALUES (`+DepositModel.Placeholders+`)
         RETURNING id`,
        dep,
    ).Scan(&dep.Id)
    return dep, err
}

// NOTE: This should be called in a serializable transaction to ensure that
// the user's balance gets updated in a safe manner.
func UpdateDepositSetStatus(tx *db.ModelTx, depositId int64, status int32) {
    updated := time.Now().Unix()
    _, err := tx.Exec(
        `UPDATE account_deposit
         SET status=?, updated=?
         WHERE id=?`,
        status, updated, depositId,
    )
    if err != nil { panic(err) }
}

func LoadDeposit(c db.MConn, depositId int64) (*Deposit) {
    var dep Deposit
    err := c.QueryRow(
        `SELECT `+DepositModel.FieldsSimple+`
         FROM account_deposit
         WHERE id=?`,
        depositId,
    ).Scan(&dep)
    if err != nil { panic(err) }
    return &dep
}

func LoadDepositsByWalletAndCoin(userId int64, wallet string, coin string, limit uint) []*Deposit {
    rows, err := db.QueryAll(Deposit{},
        `SELECT `+DepositModel.FieldsSimple+`
         FROM account_deposit
         WHERE user_id=? AND wallet=? AND coin=?
         ORDER BY time DESC LIMIT ?`,
        userId, wallet, coin, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Deposit)
}

func LoadDepositForPayment(c db.MConn, paymentId int64) (*Deposit) {
    var dep Deposit
    err := c.QueryRow(
        `SELECT `+DepositModel.FieldsSimple+`
         FROM account_deposit
         WHERE payment_id=?`,
        paymentId,
    ).Scan(&dep)
    if err != nil { panic(err) }
    return &dep
}

// WITHDRAWAL

type Withdrawal struct {
    Id          int64   `json:"id"              db:"id,autoinc"`
    UserId      int64   `json:"userId"          db:"user_id"`
    Wallet      string  `json:"wallet"          db:"wallet"`
    Coin        string  `json:"coin"            db:"coin"`
    ToAddress   string  `json:"toAddress"       db:"to_address"`
    Amount      uint64  `json:"amount"          db:"amount"`
    Approved    int32   `json:"approved"        db:"approved"`
    Status      int32   `json:"status"          db:"status"`
    WTxId       int64   `json:"wtxId"           db:"wtx_id"`
    Time        int64   `json:"time"            db:"time"`
    Updated     int64   `json:"updated"         db:"updated"`
}

var WithdrawalModel = db.GetModelInfo(new(Withdrawal))

const (
    WITHDRAWAL_STATUS_NULL = 0
    WITHDRAWAL_STATUS_PENDING = 1
    WITHDRAWAL_STATUS_CHECKEDOUT = 2
    WITHDRAWAL_STATUS_COMPLETE = 3
    WITHDRAWAL_STATUS_STALLED = 4
    WITHDRAWAL_STATUS_CANCELED = 5
)

func SaveWithdrawal(c db.MConn, wth *Withdrawal) (*Withdrawal) {
    if wth.Time == 0 { wth.Time = time.Now().Unix() }
    // Add to DB
    err := c.QueryRow(
        `INSERT INTO account_withdrawal (`+WithdrawalModel.FieldsInsert+`)
         VALUES (`+WithdrawalModel.Placeholders+`)
         RETURNING id`,
        wth,
    ).Scan(&wth.Id)
    if err != nil { panic(err) }
    return wth
}

func LoadWithdrawal(c db.MConn, id int64) *Withdrawal {
    var wth Withdrawal
    err := c.QueryRow(
        `SELECT `+WithdrawalModel.FieldsSimple+`
         FROM account_withdrawal
         WHERE id=?`,
        id,
    ).Scan(&wth)
    if err != nil { panic(err) }
    return &wth
}

func LoadWithdrawals(limit uint) []*Withdrawal {
    rows, err := db.QueryAll(Withdrawal{},
        `SELECT `+WithdrawalModel.FieldsSimple+`
         FROM account_withdrawal
         ORDER BY id DESC LIMIT ?`,
        limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Withdrawal)
}

func LoadWithdrawalsByCoin(coin string, limit uint) []*Withdrawal {
    rows, err := db.QueryAll(Withdrawal{},
        `SELECT `+WithdrawalModel.FieldsSimple+`
         FROM account_withdrawal
         WHERE coin=?
         ORDER BY id DESC LIMIT ?`,
        coin, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Withdrawal)
}

func LoadWithdrawalsByStatus(c db.MConn, coin string, status int32, limit uint) []*Withdrawal {
    rows, err := c.QueryAll(Withdrawal{},
        `SELECT `+WithdrawalModel.FieldsSimple+`
         FROM account_withdrawal
         WHERE status=? AND coin=?
         ORDER BY id ASC LIMIT ?`,
        status, coin, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Withdrawal)
}

func LoadWithdrawalsByUser(userId int64, coin string, limit uint) []*Withdrawal {
    rows, err := db.QueryAll(Withdrawal{},
        `SELECT `+WithdrawalModel.FieldsSimple+`
         FROM account_withdrawal
         WHERE user_id=? AND coin=?
         ORDER BY id DESC LIMIT ?`,
        userId, coin, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Withdrawal)
}

func UpdateWithdrawals(tx *db.ModelTx, wthIds[]interface{}, oldStatus, newStatus int, wtxId int64) {
    if len(wthIds) == 0 { return }

    res, err := tx.Exec(
        `UPDATE account_withdrawal
         SET status=?, wtx_id=?, updated=?
         WHERE status=? AND id IN (`+Placeholders(len(wthIds))+`)`,
        append([]interface{}{newStatus, wtxId, time.Now().Unix(), oldStatus}, wthIds...)...,
    )
    if err != nil { panic(err) }
    count, err := res.RowsAffected()
    if err != nil { panic(err) }
    if int(count) != len(wthIds) {
        panic(NewError("Unexpected affected rows count: %v Expected %v", count, len(wthIds)))
    }
}

// TRANSFER
// NOTE: not used anymore/yet.

type Transfer struct {
    Id          int64   `json:"id"              db:"id,autoinc"`
    Type        string  `json:"type"            db:"type"`
    UserId      int64   `json:"userId"          db:"user_id"`
    Wallet      string  `json:"wallet"          db:"wallet"`
    User2Id     int64   `json:"user2Id"         db:"user2_id,null"`
    Wallet2     string  `json:"wallet2"         db:"wallet2,null"`
    Coin        string  `json:"coin"            db:"coin"`
    Amount      uint64  `json:"amount"          db:"amount"`
    Fee         uint64  `json:"fee"             db:"fee"`
    Time        int64   `json:"time"            db:"time"`
}

var TransferModel = db.GetModelInfo(new(Transfer))

func SaveTransfer(c db.MConn, trans *Transfer) (*Transfer) {
    if trans.Time == 0 { trans.Time = time.Now().Unix() }
    err := c.QueryRow(
        `INSERT INTO account_transfer (`+TransferModel.FieldsInsert+`)
         VALUES (`+TransferModel.Placeholders+`)
         RETURNING id`,
        trans,
    ).Scan(&trans.Id)
    if err != nil { panic(err) }
    return trans
}
