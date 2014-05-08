package db

import (
    "database/sql"
    . "ftnox.com/common"
)

var migrations = []func() error{
    migrateCreateUser,
    migrateCreateAPIKey,
    migrateCreateMPK,
    migrateCreateAddress,
    migrateCreatePayment,
    migrateCreateBlock,
    migrateCreateKVStore,
    migrateCreateOutgoingEmail,
    migrateCreateAccountBalance,
    migrateCreateAccountDeposit,
    migrateCreateAccountWithdrawal,
    migrateCreateAccountTransfer,
    migrateCreateWithdrawalTx,
    migrateCreateOrder,
    migrateCreateOrderFunction,
    migrateCreateTrade,
    migrateCreatePriceLog,
    migrateCreateBetaSignup,
    //migrateCreateBankWithdrawal,
}

func migrateDb() {
    // Create the table, if needed
    _, err := Exec(`CREATE TABLE IF NOT EXISTS migration (version INT NOT NULL)`)
    if err != nil {
        panic(err)
    }

    // What version are we at? lock it
    var version int
    err = QueryRow("SELECT version FROM migration").Scan(&version)
    if err == sql.ErrNoRows {
        version = 0
    } else if err != nil {
        panic(err)
    }

    // Apply migrations
    for ; version < len(migrations); version++ {
        Info("Migrating DB version %d to %d", version, version+1)
        err = migrations[version]()
        if err != nil { panic(err) }
        if version == 0 {
            _, err = Exec("INSERT INTO migration (version) VALUES (?)", version+1)
            if err != nil { panic(err) }
        } else {
            _, err = Exec("UPDATE migration SET version=?", version+1)
            if err != nil { panic(err) }
        }
    }
}

func migrateCreateUser() error {
    _, err := Exec(`CREATE TABLE auth_user (
        id              BIGSERIAL,
        email           VARCHAR(256) NOT NULL,
        email_code      CHAR(24)     NOT NULL,
        email_conf      INT          NOT NULL,
        scrypt          BYTEA        NOT NULL,
        salt            BYTEA        NOT NULL,
        totp_key        BYTEA        NOT NULL,
        totp_conf       INT          NOT NULL,
        chain_idx       INT          NOT NULL,
        roles           VARCHAR(256) NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE auth_user_id_seq START WITH 1;
    CREATE UNIQUE INDEX auth_user_chain_idx  ON auth_user (chain_idx);
    CREATE UNIQUE INDEX auth_user_email      ON auth_user (email);
    CREATE UNIQUE INDEX auth_user_email_code ON auth_user (email_code);
    `)
    return err
}

func migrateCreateAPIKey() error {
    _, err := Exec(`CREATE TABLE auth_api_key (
        key             CHAR(24)     NOT NULL,
        user_id         BIGINT       NOT NULL,
        roles           VARCHAR(256) NOT NULL,

        PRIMARY KEY (key)
    );
    CREATE INDEX ON auth_api_key (user_id);
    `)
    return err
}

func migrateCreateMPK() error {
    _, err := Exec(`CREATE TABLE mpk (
        id      BIGSERIAL,
        pubkey  VARCHAR(128) NOT NULL,
        chain   VARCHAR(64)  NOT NULL,

        PRIMARY KEY(id)
    );
    ALTER SEQUENCE mpk_id_seq START WITH 1;
    CREATE UNIQUE INDEX mpk_pubkey ON mpk (pubkey);
    `)
    return err
}

func migrateCreateAddress() error {
    _, err := Exec(`CREATE TABLE address (
        address     VARCHAR(34)     NOT NULL,
        coin        VARCHAR(4)      NOT NULL,
        user_id     BIGINT          NOT NULL,
        wallet      VARCHAR(12)     NOT NULL,
        mpk_id      BIGINT          NOT NULL,
        chain_path  VARCHAR(128)    NOT NULL,
        chain_idx   INT             NOT NULL,
        time        BIGINT          NOT NULL,

        PRIMARY KEY (address)
    );
    CREATE UNIQUE INDEX ON address (mpk_id, chain_path, chain_idx, coin);
    CREATE        INDEX ON address (user_id, wallet, coin, chain_idx);
    `)
    return err
}

func migrateCreatePayment() error {
    _, err := Exec(`CREATE TABLE payment (
        id          BIGSERIAL,
        coin        VARCHAR(4)      NOT NULL,
        tx_id       CHAR(64)        NOT NULL,
        vout        INT             NOT NULL,
        blockhash   CHAR(64)        NOT NULL,
        blockheight INT             NOT NULL, 
        address     VARCHAR(34)     NOT NULL,
        amount      BIGINT          NOT NULL,
        script_pk   VARCHAR(134)    NOT NULL,
        mpk_id      BIGINT          NOT NULL,
        spent       INT             NOT NULL,
        wtx_id      BIGINT,
        orphaned    INT             NOT NULL,
        time        BIGINT          NOT NULL,
        updated     BIGINT          NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE payment_id_seq START WITH 1;
    CREATE UNIQUE INDEX payment_tx_id_vout ON payment (tx_id, vout);
    CREATE INDEX ON payment (mpk_id, coin, spent, amount);
    CREATE INDEX ON payment (address, time);
    CREATE INDEX ON payment (blockhash);
    CREATE INDEX ON payment (wtx_id);
    `)
    return err
}

func migrateCreateBlock() error {
    _, err := Exec(`CREATE TABLE block (
        coin        VARCHAR(4)  NOT NULL,
        height      INT         NOT NULL, 
        hash        CHAR(64)    NOT NULL,
        status      INT         NOT NULL,
        time        BIGINT      NOT NULL,
        updated     BIGINT      NOT NULL,

        PRIMARY KEY (hash)
    );
    CREATE INDEX ON block (coin, height);
    `)
    return err
}

func migrateCreateKVStore() error {
    _, err := Exec(`CREATE TABLE kvstore (
        key_        VARCHAR(128)  NOT NULL,
        value       VARCHAR(1024) NOT NULL,

        PRIMARY KEY (key_)
    )`)
    return err
}

func migrateCreateOutgoingEmail() error {
    _, err := Exec(`CREATE TABLE outgoing_email (
        message_id  VARCHAR(64)     NOT NULL,
        from_       VARCHAR(256)    NOT NULL,
        to_         VARCHAR(256)    NOT NULL,
        subject     VARCHAR(1024)   NOT NULL,
        body_plain  TEXT            NOT NULL,
        time_sent   BIGINT          NOT NULL,
        error       VARCHAR(1024)   NOT NULL,

        PRIMARY KEY (message_id)
    );
    CREATE INDEX ON outgoing_email (time_sent);
    `)
    return err
}

func migrateCreateAccountBalance() error {
    _, err := Exec(`CREATE TABLE account_balance (
        user_id     BIGINT      NOT NULL,
        wallet      VARCHAR(12) NOT NULL,
        coin        VARCHAR(4)  NOT NULL,
        amount      BIGINT      NOT NULL,

        PRIMARY KEY (user_id, wallet, coin)
    )`)
    return err
}

func migrateCreateAccountDeposit() error {
    _, err := Exec(`CREATE TABLE account_deposit (
        id              BIGSERIAL,
        type            CHAR(1)     NOT NULL,
        user_id         BIGINT      NOT NULL,
        wallet          VARCHAR(12) NOT NULL,
        coin            VARCHAR(4)  NOT NULL,
        amount          BIGINT      NOT NULL,
        payment_id      BIGINT,
        status          INT         NOT NULL,
        time            BIGINT      NOT NULL,
        updated         BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE account_deposit_id_seq START WITH 1;
    CREATE UNIQUE INDEX ON account_deposit (payment_id) WHERE payment_id IS NOT NULL;
    CREATE INDEX ON account_deposit (user_id, wallet, coin, time);
    CREATE INDEX ON account_deposit (user_id, wallet, time);
    `)
    return err
}

func migrateCreateAccountWithdrawal() error {
    _, err := Exec(`CREATE TABLE account_withdrawal (
        id          BIGSERIAL,
        user_id     BIGINT      NOT NULL,
        wallet      VARCHAR(12) NOT NULL,
        coin        VARCHAR(4)  NOT NULL,
        to_address  VARCHAR(34) NOT NULL,
        amount      BIGINT      NOT NULL,
        approved    INT         NOT NULL,
        status      INT         NOT NULL,
        wtx_id      BIGINT,
        time        BIGINT      NOT NULL,
        updated     BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE account_withdrawal_id_seq START WITH 1;
    CREATE INDEX ON account_withdrawal (coin, id);
    CREATE INDEX ON account_withdrawal (status, coin, id);
    CREATE INDEX ON account_withdrawal (wtx_id);
    `)
    return err
}

func migrateCreateAccountTransfer() error {
    _, err := Exec(`CREATE TABLE account_transfer (
        id              BIGSERIAL,
        type            CHAR(1)     NOT NULL,
        user_id         BIGINT      NOT NULL,
        wallet          VARCHAR(12) NOT NULL,
        user2_id        BIGINT      NOT NULL,
        wallet2         VARCHAR(12) NOT NULL,
        coin            VARCHAR(4)  NOT NULL,
        amount          BIGINT      NOT NULL,
        fee             BIGINT      NOT NULL,
        time            BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE account_transfer_id_seq START WITH 1;
    CREATE INDEX ON account_transfer (type, user_id, time);
    CREATE INDEX ON account_transfer (type, user2_id, time) WHERE user2_id IS NOT NULL;
    `)
    return err
}

func migrateCreateWithdrawalTx() error {
    _, err := Exec(`CREATE TABLE withdrawal_tx (
        id          BIGSERIAL,
        type        CHAR(1)     NOT NULL,
        coin        VARCHAR(4)  NOT NULL,
        from_mpk_id BIGINT,
        to_mpk_id   BIGINT,
        amount      BIGINT      NOT NULL,
        miner_fee   BIGINT      NOT NULL,
        chg_address VARCHAR(34) NOT NULL,
        raw_tx      TEXT        NOT NULL,
        tx_id       TEXT        NOT NULL,
        time        BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE withdrawal_tx_id_seq START WITH 1;
    CREATE INDEX ON withdrawal_tx (coin, time);
    `)
    return err
}

func migrateCreateOrder() error {
    _, err := Exec(`CREATE TABLE exchange_order (
        id              BIGSERIAL,
        type            CHAR(1)     NOT NULL,
        user_id         BIGINT      NOT NULL,
        coin            VARCHAR(4)  NOT NULL,
        amount          BIGINT      NOT NULL,
        filled          BIGINT      NOT NULL,
        basis_coin      VARCHAR(4)  NOT NULL,
        basis_amount    BIGINT      NOT NULL,
        basis_filled    BIGINT      NOT NULL,
        basis_fee       BIGINT      NOT NULL,
        basis_fee_filled BIGINT     NOT NULL,
        basis_fee_ratio DOUBLE PRECISION NOT NULL,
        price           DOUBLE PRECISION NOT NULL,
        status          INT         NOT NULL,
        time            BIGINT      NOT NULL,
        updated         BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE exchange_order_id_seq START WITH 1;
    CREATE INDEX ON exchange_order (status, basis_coin, coin, price) WHERE status = 0;
    CREATE INDEX ON exchange_order (status, user_id, basis_coin, coin) WHERE status = 0;
    `)
    return err
}

func migrateCreateTrade() error {
    _, err := Exec(`CREATE TABLE exchange_trade (
        id              BIGSERIAL,
        bid_user_id     BIGINT      NOT NULL,
        bid_order_id    BIGINT      NOT NULL,
        bid_basis_fee   BIGINT      NOT NULL,
        ask_user_id     BIGINT      NOT NULL,
        ask_order_id    BIGINT      NOT NULL,
        ask_basis_fee   BIGINT      NOT NULL,
        coin            VARCHAR(4)  NOT NULL,
        basis_coin      VARCHAR(4)  NOT NULL,
        trade_amount    BIGINT      NOT NULL,
        trade_basis     BIGINT      NOT NULL,
        price           DOUBLE PRECISION NOT NULL,
        time            BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE exchange_trade_id_seq START WITH 1;
    CREATE INDEX ON exchange_trade (bid_user_id, basis_coin, coin, time);
    CREATE INDEX ON exchange_trade (ask_user_id, basis_coin, coin, time);
    `)
    return err
}

func migrateCreateOrderFunction() error {
    _, err := Exec(`
    CREATE OR REPLACE FUNCTION exchange_do_trade (
        bid_order_id BIGINT, bid_user_id BIGINT, bid_basis_fee BIGINT,
        ask_order_id BIGINT, ask_user_id BIGINT, ask_basis_fee BIGINT,
        basis_coin VARCHAR(4), basis_amount BIGINT,
        other_coin VARCHAR(4), other_amount BIGINT) RETURNS VOID AS
    $$
        BEGIN
            -- Subtract coins from both reserve wallets, and ensure that funds exist
            IF (SELECT amount FROM account_balance WHERE user_id=bid_user_id AND wallet='reserved_o' AND coin=basis_coin) >= (basis_amount + bid_basis_fee) THEN 
                UPDATE account_balance SET amount = amount - (basis_amount + bid_basis_fee) WHERE user_id=bid_user_id AND wallet='reserved_o' AND coin=basis_coin;
            ELSE
                RAISE EXCEPTION 'Not enough funds (needed %) for bid_user_id: (%), bid_order_id: (%), ask_order_id: (%)', (basis_amount + bid_basis_fee), bid_user_id, bid_order_id, ask_order_id;
            END IF;
            IF (SELECT amount FROM account_balance WHERE user_id=ask_user_id AND wallet='reserved_o' AND coin=other_coin) >= other_amount THEN 
                UPDATE account_balance SET amount = amount - other_amount WHERE user_id=ask_user_id AND wallet='reserved_o' AND coin=other_coin;
            ELSE
                RAISE EXCEPTION 'Not enough funds (needed %) for ask_user_id: (%), bid_order_id: (%), ask_order_id: (%)', other_amount, ask_user_id, bid_order_id, ask_order_id;
            END IF;

            -- Add funds to bid_user's main wallet
            UPDATE account_balance SET amount = amount + other_amount WHERE user_id=bid_user_id AND wallet='main' AND coin=other_coin;
            IF NOT FOUND THEN
                INSERT INTO account_balance (user_id, wallet, coin, amount) VALUES (bid_user_id, 'main', other_coin, other_amount);
            END IF;

            -- Add funds to ask_user's main wallet
            UPDATE account_balance SET amount = amount + (basis_amount - ask_basis_fee) WHERE user_id=ask_user_id AND wallet='main' AND coin=basis_coin;
            IF NOT FOUND THEN
                INSERT INTO account_balance (user_id, wallet, coin, amount) VALUES (ask_user_id, 'main', basis_coin, (basis_amount - ask_basis_fee));
            END IF;
        END;
    $$
    LANGUAGE plpgsql;
    `)
    return err
}

func migrateCreatePriceLog() error {
    _, err := Exec(`CREATE TABLE exchange_price_log (
        id              BIGSERIAL,
        market          VARCHAR(12) NOT NULL,
        low             DOUBLE PRECISION NOT NULL,
        high            DOUBLE PRECISION NOT NULL,
        open            DOUBLE PRECISION NOT NULL,
        close           DOUBLE PRECISION NOT NULL,
        interval        BIGINT      NOT NULL,
        ask_volume      BIGINT      NOT NULL,
        bid_volume      BIGINT      NOT NULL,
        time            BIGINT      NOT NULL,
        timestamp       TIMESTAMPTZ NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE exchange_price_log_id_seq START WITH 1;
    CREATE UNIQUE INDEX ON exchange_price_log (market, interval, time);
    `)
    return err
}

func migrateCreateBetaSignup() error {
    _, err := Exec(`CREATE TABLE beta_signup (
        id              BIGSERIAL,
        body            TEXT            NOT NULL,
        header          TEXT            NOT NULL,
        time            BIGINT      NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE beta_signup_id_seq START WITH 1;
    `)
    return err
}

/*
func migrateCreateBankWithdrawal() error {
    _, err := Exec(`CREATE TABLE account_bank_withdrawal (
        id          BIGSERIAL,
        coin        VARCHAR(4)      NOT NULL,
        tx_id       CHAR(64)        NOT NULL,
        vout        INT             NOT NULL,
        blockhash   CHAR(64)        NOT NULL,
        blockheight INT             NOT NULL, 
        address     VARCHAR(34)     NOT NULL,
        user_id     BIGINT          NOT NULL,
        wallet      VARCHAR(12)     NOT NULL,
        amount      BIGINT          NOT NULL,
        script_pk   VARCHAR(134)    NOT NULL,
        mpk_id      BIGINT          NOT NULL,
        spent       INT             NOT NULL,
        wtx_id      BIGINT,
        orphaned    INT             NOT NULL,
        credited    INT             NOT NULL,
        time        BIGINT          NOT NULL,
        updated     BIGINT          NOT NULL,

        PRIMARY KEY (id)
    );
    ALTER SEQUENCE payment_id_seq START WITH 1;
    CREATE UNIQUE INDEX payment_tx_id_vout ON payment (tx_id, vout);
    CREATE INDEX ON payment (mpk_id, coin, spent, amount);
    CREATE INDEX ON payment (user_id, wallet, coin, time);
    CREATE INDEX ON payment (user_id, wallet, time);
    CREATE INDEX ON payment (address, time);
    CREATE INDEX ON payment (blockhash);
    CREATE INDEX ON payment (spent);
    CREATE INDEX ON payment (wtx_id);
    `)
    return err
}
*/
