package tests

import (
    . "ftnox.com/common"
    "ftnox.com/account"
    "testing"
)

func TestWithdrawals(t *testing.T) {
    user := GenerateRandomUser()
    DepositMoneyForUser(user, "BTC", USATOSHI)

    EnsureBalances(t, user.Id, account.WALLET_MAIN, map[string]int64{
        "BTC": SATOSHI,
    })

    // Add a withdrawal to some address. We'll choose the
    // user's deposit address for this test.
    address := account.LoadOrCreateDepositAddress(user.Id, account.WALLET_MAIN, "BTC")
    wth, err := account.AddWithdrawal(user.Id, address, "BTC", USATOSHI)
    if err != nil { t.Fatal("Unexpected error from AddWithdrawal", err) }

    // Ensure that the balance for the withdrawal moved to WALLET_RESERVED_WITHDRAWAL// Ensure that the balance for the withdrawal moved to WALLET_RESERVED_WITHDRAWAL
    EnsureBalances(t, user.Id, account.WALLET_MAIN, map[string]int64{
        "BTC": 0,
    })
    EnsureBalances(t, user.Id, account.WALLET_RESERVED_WITHDRAWAL, map[string]int64{
        "BTC": SATOSHI,
    })

    // Cancel the withdrawal
    account.CancelWithdrawal(wth)

    // Ensure that all the funds moved back to WALLET_MAIN
    EnsureBalances(t, user.Id, account.WALLET_MAIN, map[string]int64{
        "BTC": SATOSHI,
    })
    EnsureBalances(t, user.Id, account.WALLET_RESERVED_WITHDRAWAL, map[string]int64{
        "BTC": 0,
    })
}
