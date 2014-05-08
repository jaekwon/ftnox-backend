package bitcoin

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "time"
    "strings"
    "errors"
    "database/sql"
)

//////////// MPK
//////////// MPK

type MPK struct {
    Id      int64  `json:"id"       db:"id,autoinc"`
    PubKey  string `json:"pubkey"   db:"pubkey"`
    Chain   string `json:"chain"    db:"chain"`
}

var MPKModel = db.GetModelInfo(new(MPK))

func SaveMPK(mpk *MPK) (*MPK) {
    err := db.QueryRow(
        `INSERT INTO mpk (`+MPKModel.FieldsInsert+`)
         VALUES (`+MPKModel.Placeholders+`)
         RETURNING id`,
        mpk,
    ).Scan(&mpk.Id)
    if err != nil { panic(err) }
    return mpk
}

func SaveMPKIfNotExists(mpk *MPK) (*MPK) {
    // Insert MPK if doesn't exist.
    mpk_ := LoadMPKByPubKey(mpk.PubKey)
    if mpk_ != nil {
        if mpk_.Chain != mpk.Chain {
            panic(errors.New("Loaded account MPK but chain did not match"))
        } else {
            return mpk_
        }
    }
    return SaveMPK(mpk)
}

func LoadMPK(mpkId int64) (*MPK) {
    var mpk MPK
    err := db.QueryRow(
        `SELECT `+MPKModel.FieldsSimple+`
         FROM mpk WHERE id=?`,
        mpkId,
    ).Scan(&mpk)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &mpk
    default:
        panic(err)
    }
}

func LoadMPKByPubKey(pubKey string) (*MPK) {
    var mpk MPK
    err := db.QueryRow(
        `SELECT `+MPKModel.FieldsSimple+`
         FROM mpk WHERE pubkey=?`,
        pubKey,
    ).Scan(&mpk)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &mpk
    default:
        panic(err)
    }
}

//////////// ADDRESS
//////////// ADDRESS

// ChainPath starting with 1/... is a change address.
type Address struct {
    Address     string `json:"address"      db:"address"`
    Coin        string `json:"coin"         db:"coin"`
    UserId      int64  `json:"userId"       db:"user_id"`
    Wallet      string `json:"wallet"       db:"wallet"`
    MPKId       int64  `json:"mpkId"        db:"mpk_id"`
    ChainPath   string `json:"chainPath"    db:"chain_path"`
    ChainIdx    int32  `json:"chainIdx"     db:"chain_idx"`
    Time        int64  `json:"time"         db:"time"`
}

var AddressModel = db.GetModelInfo(new(Address))

func SaveAddress(addr *Address) (*Address, error) {
    _, err := db.Exec(
        `INSERT INTO address (`+AddressModel.FieldsInsert+`)
         VALUES (`+AddressModel.Placeholders+`)`,
        addr,
    )
    return addr, err
}

func LoadAddress(address string) (*Address) {
    var addr Address
    err := db.QueryRow(
        `SELECT `+AddressModel.FieldsSimple+`
         FROM address
         WHERE address=?`,
        address,
    ).Scan(&addr)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &addr
    default:
        panic(err)
    }
}

func LoadAddressesByWallet(userId int64, wallet string) ([]*Address) {
    rows, err := db.QueryAll(Address{},
        `SELECT `+AddressModel.FieldsSimple+`
         FROM address
         WHERE user_id=? AND wallet=?`,
        userId, wallet,
    )
    if err != nil { panic(err) }
    return rows.([]*Address)
}

func LoadLastAddressByWallet(userId int64, wallet string, coin string) (*Address) {
    var addr Address
    err := db.QueryRow(
        `SELECT `+AddressModel.FieldsSimple+`
         FROM address
         WHERE user_id=? AND wallet=? AND coin=?
         ORDER BY chain_idx DESC LIMIT 1`,
        userId, wallet, coin,
    ).Scan(&addr)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &addr
    default:
        panic(err)
    }
}

func LoadKnownAddresses(addrStrs []string) []*Address {
    if len(addrStrs) == 0 { return nil }
    var addrStrs_i []interface{}
    for _, addrStr := range addrStrs { addrStrs_i = append(addrStrs_i, addrStr) }
    // TODO: consider limitations on placeholder count. 65536?
    addrsPH := "?" + strings.Repeat(",?", len(addrStrs)-1)
    rows, err := db.QueryAll(Address{},
        `SELECT `+AddressModel.FieldsSimple+`
         FROM address WHERE address in (`+addrsPH+`)`,
        addrStrs_i...,
    )
    if err != nil { panic(err) }
    return rows.([]*Address)
}

func LoadAddressesByMPK(mpkId int64) []*Address {
    rows, err := db.QueryAll(Address{},
        `SELECT `+AddressModel.FieldsSimple+`
         FROM address WHERE mpk_id=?
         ORDER BY (chain_path, chain_idx) ASC`,
        mpkId,
    )
    if err != nil { panic(err) }
    return rows.([]*Address)
}

func GetMaxAddressIndex(coin string, mpkId int64, chainPath string) int32 {
    var countNull sql.NullInt64
    err := db.QueryRow(
        `SELECT max(chain_idx)
         FROM address
         WHERE coin=? AND mpk_id=? AND chain_path=?`,
        coin, mpkId, chainPath,
    ).Scan(&countNull)
    if err != nil { panic(err) }
    return int32(countNull.Int64)
}

// Keeps trying until one is created in the path.
// The resulting address's path will be 'chainpath/x'
// where 'x' is the smallest nonnegative integer.
func CreateNewAddress(coin string, userId int64, wallet string, mpk *MPK, chainPath string) *Address {
    now := time.Now().Unix()
    index := GetMaxAddressIndex(coin, mpk.Id, chainPath)
    for {
        index += 1
        address := ComputeAddress(coin, mpk.PubKey, mpk.Chain, chainPath, index)
        addr := &Address{address, coin, userId, wallet, mpk.Id, chainPath, index, now}
        addr, err := SaveAddress(addr)
        Info("[%v] Created new address: wallet:%v/%v mpkId:%v chainPath:%v/%v",
            coin, userId, wallet, mpk.Id, chainPath, index)
        switch db.GetErrorType(err) {
        case db.ERR_DUPLICATE_ENTRY:
            continue
        case nil:
            return addr
        default:
            panic(err)
        }
    }
}

//////////// PAYMENT
//////////// PAYMENT

type Payment struct {
    Id          int64  `json:"-"            db:"id,autoinc"`
    Coin        string `json:"coin"         db:"coin"`
    TxId        string `json:"txid"         db:"tx_id"`
    Vout        uint32 `json:"vout"         db:"vout"`
    Blockhash   string `json:"blockhash"    db:"blockhash"`
    Blockheight uint32 `json:"blockheight"  db:"blockheight"`
    Address     string `json:"address"      db:"address"`
    Amount      uint64 `json:"amount"       db:"amount"`
    ScriptPK    string `json:"scriptPk"     db:"script_pk"`
    MPKId       int64  `json:"mpkId"        db:"mpk_id"`
    Spent       uint32 `json:"-"            db:"spent"`
    WTxId       int64  `json:"-"            db:"wtx_id"`
    Orphaned    uint32 `json:"orphaned"     db:"orphaned"`
    Confirms    uint32 `json:"confirms"`
    Time        int64  `json:"time"         db:"time"`
    Updated     int64  `json:"updated"      db:"updated"`
}

var PaymentModel = db.GetModelInfo(new(Payment))

const (
    PAYMENT_ORPHANED_STATUS_GOOD = 0
    PAYMENT_ORPHANED_STATUS_ORPHANED = 1

    PAYMENT_SPENT_STATUS_AVAILABLE = 0
    PAYMENT_SPENT_STATUS_CHECKEDOUT = 1
    PAYMENT_SPENT_STATUS_SPENT = 2
)

func SavePayment(c db.MConn, p *Payment) (*Payment, error) {
    if p.Time == 0 { p.Time = time.Now().Unix() }
    err := c.QueryRow(
        `INSERT INTO payment (`+PaymentModel.FieldsInsert+`)
         VALUES (`+PaymentModel.Placeholders+`)
         RETURNING id`,
        p,
    ).Scan(&p.Id)
    return p, err
}

func UpdatePayment(c db.MConn, p *Payment) {
    p.Updated = time.Now().Unix()
    _, err := c.Exec(
        `UPDATE payment SET blockhash=?, blockheight=?, orphaned=?, time=?, updated=? WHERE tx_id=? AND vout=?`,
        p.Blockhash, p.Blockheight, p.Orphaned, p.Time, p.Updated, p.TxId, p.Vout,
    )
    if err != nil { panic(err) }
}

func UpdatePaymentsSpent(tx *db.ModelTx, paymentIds []interface{}, oldStatus, newStatus int, wtxId int64) {
    if len(paymentIds) == 0 { return }
    now := time.Now().Unix()

    res, err := tx.Exec(
        `UPDATE payment
         SET spent=?, wtx_id=?, updated=?
         WHERE spent=? AND id IN (`+Placeholders(len(paymentIds))+`)`,
        append([]interface{}{newStatus, wtxId, now, oldStatus}, paymentIds...)...,
    )
    if err != nil { panic(err) }
    count, err := res.RowsAffected()
    if int(count) != len(paymentIds) {
        panic(NewError("Unexpected affected rows count: %v Expected %v", count, len(paymentIds)))
    }
    if err != nil { panic(err) }
}

func LoadPaymentByTxId(txId string, vout uint32) *Payment {
    var payment Payment
    err := db.QueryRow(
        `SELECT `+PaymentModel.FieldsSimple+` FROM payment
         WHERE tx_id=? AND vout=?`,
        txId, vout,
    ).Scan(&payment)
    switch err {
    case sql.ErrNoRows: return nil
    case nil:           return &payment
    default:            panic(err)
    }
}

func LoadPayments(limit uint) []*Payment {
    rows, err := db.QueryAll(Payment{},
        `SELECT `+PaymentModel.FieldsSimple+` FROM payment
         ORDER BY id DESC LIMIT ?`,
        limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Payment)
}

// Loads payments associated with a given block(hash),
// regardless of orphaned/spent status.
func LoadPaymentsByBlockhash(blockhash string) []*Payment {
    rows, err := db.QueryAll(Payment{},
        `SELECT `+PaymentModel.FieldsSimple+` FROM payment
         WHERE blockhash=?`,
        blockhash,
    )
    if err != nil { panic(err) }
    return rows.([]*Payment)
}

func LoadSpendablePaymentsByAmount(mpkId int64, coin string, min, max uint64, reqHeight uint32, limit uint) []*Payment {
    rows, err := db.QueryAll(Payment{},
        `SELECT `+PaymentModel.FieldsSimple+` FROM payment
         WHERE mpkId=? AND coin=? AND spent=0 AND orphaned=0 AND ?<=amount AND amount<=? AND blockheight>0 AND blockheight<=?
         ORDER BY amount ASC LIMIT ?`,
        mpkId, coin, min, max, reqHeight, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Payment)
}

func LoadLargestSpendablePaymentLessThan(mpkId int64, coin string, amount uint64, reqHeight uint32, exclude []*Payment) *Payment {
    excludeIds := Map(exclude, "Id")
    if len(excludeIds) == 0 { excludeIds = []interface{}{-1} } // hack
    var payment Payment
    err := db.QueryRow(
        `SELECT `+PaymentModel.FieldsSimple+`
         FROM payment WHERE
         mpk_id=? AND coin=? AND spent=0 AND orphaned=0 AND amount<=? AND blockheight>0 AND blockheight<=?
         AND id NOT IN (`+Placeholders(len(excludeIds))+`)`,
        append([]interface{}{mpkId, coin, amount, reqHeight}, excludeIds...)...).Scan(&payment)
    switch err {
    case sql.ErrNoRows: return nil
    case nil:           return &payment
    default:            panic(err)
    }
}

func LoadSmallestSpendablePaymentGreaterThan(mpkId int64, coin string, amount uint64, reqHeight uint32, exclude []*Payment) *Payment {
    excludeIds := Map(exclude, "Id")
    if len(excludeIds) == 0 { excludeIds = []interface{}{-1} } // hack
    var payment Payment
    err := db.QueryRow(
        `SELECT `+PaymentModel.FieldsSimple+`
         FROM payment WHERE
         mpk_id=? AND coin=? AND spent=0 AND orphaned=0 AND amount>=? AND blockheight>0 AND blockheight<=?
         AND id NOT IN (`+Placeholders(len(excludeIds))+`)`,
        append([]interface{}{mpkId, coin, amount, reqHeight}, excludeIds...)...).Scan(&payment)
    switch err {
    case sql.ErrNoRows: return nil
    case nil:           return &payment
    default:            panic(err)
    }
    return &payment
}

func LoadOldestSpendablePaymentsBetween(mpkId int64, coin string, min, max uint64, limit int, reqHeight uint32) []*Payment {
    rows, err := db.QueryAll(Payment{},
        `SELECT `+PaymentModel.FieldsSimple+`
         FROM payment
         WHERE mpk_id=? AND coin=? AND spent=0 AND orphaned=0 AND ?<=amount AND amount<=? AND blockheight>0 AND blockheight<=?
         ORDER BY id ASC LIMIT ?`,
        mpkId, coin, min, max, reqHeight, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Payment)
}

// NOTE: only use this for gathering statistical data. Try really hard not to introduce randomness into the system, e.g. sweep transactions.
func LoadRandomSpendablePaymentsBetween(mpkId int64, coin string, min, max uint64, limit int, reqHeight uint32) []*Payment {
    rows, err := db.QueryAll(Payment{},
        `SELECT `+PaymentModel.FieldsSimple+`
         FROM payment
         WHERE mpk_id=? AND coin=? AND spent=0 AND orphaned=0 AND ?<=amount AND amount<=? AND blockheight>0 AND blockheight<=?
         ORDER BY random() LIMIT ?`,
        mpkId, coin, min, max, reqHeight, limit,
    )
    if err != nil { panic(err) }
    return rows.([]*Payment)
}


//////////// BLOCK
//////////// BLOCK

type Block struct {
    Coin            string  `db:"coin"`
    Height          uint32  `db:"height"`
    Hash            string  `db:"hash"`
    Status          uint32  `db:"status"`
    Time            int64   `db:"time"`
    Updated         int64   `db:"updated"`
}

var BlockModel = db.GetModelInfo(new(Block))

const (
    BLOCK_STATUS_GOOD = 0           // all payments are good.
    BLOCK_STATUS_PROCESSING = 1     // was transistioning from GOOD -> ORPHANED or ORPHANED -> GOOD.
    BLOCK_STATUS_ORPHANED = 2       // all payments are orphaned.
    BLOCK_STATUS_GOOD_CREDITED = 10 // block is good and deposits were credited.
)

func SaveBlock(b *Block) *Block {
    if b.Time == 0 { b.Time = time.Now().Unix() }
    _, err := db.Exec(
        `INSERT INTO block (`+BlockModel.FieldsInsert+`)
         VALUES (`+BlockModel.Placeholders+`)`,
        b,
    )
    if err != nil { panic(err) }
    return b
}

func LoadBlock(hash string) *Block {
    var block Block
    err := db.QueryRow(
        `SELECT `+BlockModel.FieldsSimple+`
         FROM block WHERE hash=?`,
        hash,
    ).Scan(&block)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &block
    default:
        panic(err)
    }
}

func LoadBlockAtHeight(coin string, height uint32) *Block {
    var block Block
    err := db.QueryRow(
        `SELECT `+BlockModel.FieldsSimple+`
         FROM block WHERE coin=? AND height=? AND (status=0 OR status=1 OR status=10)`,
        coin, height,
    ).Scan(&block)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &block
    default:
        panic(err)
    }
}

func LoadLastBlocks(coin string, n uint32) []*Block {
    rows, err := db.QueryAll(Block{},
        `SELECT `+BlockModel.FieldsSimple+` FROM block
         WHERE coin=? AND (status=0 OR status=1 OR status=10)
         ORDER BY height DESC LIMIT ?`,
        coin, n,
    )
    if err != nil { panic(err) }
    return rows.([]*Block)
}

func UpdateBlockStatus(hash string, oldStatus, newStatus uint32) {
    now := time.Now().Unix()
    res, err := db.Exec(
        `UPDATE block
         SET status=?, updated=?
         WHERE status=? AND hash=?`,
        newStatus, now, oldStatus, hash,
    )
    if err != nil { panic(err) }
    count, err := res.RowsAffected()
    if int(count) != 1 {
        panic(NewError("Expected to update 1 block's status, but none changed"))
    }
    if err != nil { panic(err) }
}
