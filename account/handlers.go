package account

import (
    . "ftnox.com/common"
    "ftnox.com/bitcoin"
    "ftnox.com/auth"
    "net/http"
    "fmt"
)

func BalanceHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    balances := LoadBalances(user.Id, WALLET_MAIN)
    ReturnJSON(API_OK, balances)
}

func DepositAddressHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    coin :=   GetParamRegexp(r, "coin",   RE_COIN,      true)
    addr := LoadOrCreateDepositAddress(user.Id, WALLET_MAIN, coin)
    ReturnJSON(API_OK, addr)
}

func DepositsHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    coin :=     GetParamRegexp(r, "coin",       RE_COIN,    false)
    deposits := LoadDepositsByWalletAndCoin(user.Id, WALLET_MAIN, coin, 10)
    ReturnJSON(API_OK, deposits)
}

func WithdrawHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    toAddress :=        GetParamRegexp(r, "to_address",  RE_ADDRESS,    true)
    coin :=             GetParamRegexp(r, "coin",        RE_COIN,       true)
    amount :=           GetParamUint64(r, "amount")

    minWithdraw := bitcoin.MinWithdrawAmount(coin)

    if amount <= minWithdraw {
        ReturnJSON(API_INVALID_PARAM, fmt.Sprintf("Minimum withdrawal amount for %v is %v", coin, UI64ToF64(minWithdraw)))
    }

    _, err := AddWithdrawal(user.Id, toAddress, coin, amount)
    switch err {
    case nil:                       break
    case INSUFFICIENT_FUNDS_ERROR:  ReturnJSON(API_INSUFFICIENT_FUNDS, "Insufficient funds")
    default:                        panic(err)
    }

    balances := LoadBalances(user.Id, WALLET_MAIN)
    ReturnJSON(API_OK, balances)
}

func WithdrawalsHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    coin := GetParamRegexp(r, "coin", RE_COIN, true)
    withdrawals := LoadWithdrawalsByUser(user.Id, coin, 10)
    ReturnJSON(API_OK, withdrawals)
}
