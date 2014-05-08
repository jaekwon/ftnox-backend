/*
Common testing utility functions such as setting up users with account balances, etc.
*/

package tests

import (
    . "ftnox.com/common"
    "ftnox.com/auth"
    "ftnox.com/account"
    "ftnox.com/db"
    "testing"
    "fmt"
)

func GenerateRandomUser() *auth.User {
    email := fmt.Sprintf("test+%v@ftnox.com", RandId(12))
    user := &auth.User {
        Email:  email,
        Scrypt: []byte{0},
        Salt:   []byte{0},
    }
    _, err := auth.SaveUser(user)
    if err != nil { panic(err) }
    return user
}

func DepositMoneyForUser(user *auth.User, coin string, amount uint64) {
    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        account.UpdateBalanceByWallet(tx, user.Id, account.WALLET_MAIN, coin, int64(amount), false)
    })
    if err != nil { panic(err) }
}

func EnsureBalances(t *testing.T, userId int64, wallet string, expected map[string]int64) {
    balances := account.LoadBalances(userId, wallet)
    for coin, amount := range balances {
        if expected[coin] != amount {
            t.Errorf("Expected %v %v but got %v", expected[coin], coin, amount)
        }
    }
    for coin, amount := range expected {
        if amount != 0 && balances[coin] == 0 {
            t.Errorf("Expected %v %v but got %v", amount, coin, balances[coin])
        }
    }
}
