package treasury

import (
    . "ftnox.com/common"
    "ftnox.com/account"
    "ftnox.com/bitcoin"
    "ftnox.com/auth"
    "net/http"
    "strings"
    "fmt"
)

const UNAUTH_MSG = "Incidence reported" // not really


//
// SERVE HTML, CSS, JS, but only to authorized users.
//

func StaticHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

	var path string
    if strings.HasPrefix(r.URL.Path, "/treasury") { path = r.URL.Path[9:] }
	if strings.HasSuffix(path, "/") {
		path = path + "index.html"
	} else {
		path = path
	}
	http.ServeFile(w, r, "static/treasury/"+path)
}

func StorePrivateKeyHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

    pubKey :=   GetParam(r, "pub_key")
    privKey :=  GetParam(r, "priv_key")
    StorePrivateKeyForMPKPubKey(pubKey, privKey)
    ReturnJSON(API_OK, nil)
}

func CreditUserHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

    coin :=     GetParamRegexp(r, "coin", RE_COIN,  true)
    email :=    GetParamRegexp(r, "email", RE_EMAIL, true)
    amountFloat :=  GetParamFloat64(r, "amountFloat")
    amount := F64ToUI64(amountFloat)

    target := auth.LoadUserByEmail(email)
    if target == nil { ReturnJSON(API_INVALID_PARAM, fmt.Sprintf("User with email %v doesn't exist", email)) }
    if amount == 0 { ReturnJSON(API_INVALID_PARAM, "Amount cannot be zero") }

    deposit := account.CreateDeposit(&account.Deposit{
        Type:       account.DEPOSIT_TYPE_FIAT,
        UserId:     target.Id,
        Wallet:     "main",
        Coin:       coin,
        Amount:     amount,
        Status:     account.DEPOSIT_STATUS_PENDING,
    })
    account.CreditDeposit(deposit)

    ReturnJSON(API_OK, nil)
}

func GetWithdrawalsHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

    coin :=     GetParamRegexp(r, "coin", RE_COIN,  false)
    limit :=    GetParamInt32(r, "limit")
    if coin == "" {
        withdrawals := account.LoadWithdrawals(uint(limit))
        ReturnJSON(API_OK, withdrawals)
    } else {
        withdrawals := account.LoadWithdrawalsByCoin(coin, uint(limit))
        ReturnJSON(API_OK, withdrawals)
    }
}

func ResumeWithdrawalHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

    wthId := GetParamInt64(r, "withdrawalId")
    account.ResumeWithdrawals([]interface{}{wthId})
    ReturnJSON(API_OK, nil)
}

func GetDepositsHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

    limit :=    GetParamInt32(r, "limit")
    deposits := bitcoin.LoadPayments(uint(limit))
    ReturnJSON(API_OK, deposits)
}

func GetSpendablePayments(w http.ResponseWriter, r *http.Request, user *auth.User) {
    if !user.HasRole("treasury") { ReturnJSON(API_UNAUTHORIZED, UNAUTH_MSG) } // All handlers here should have this.

    mpkId :=    GetParamInt64(r,  "mpk_id")
    coin :=     GetParamRegexp(r, "coin", RE_COIN,  false)
    min :=      GetParamUint64(r, "min")
    max :=      GetParamUint64(r, "max")
    limit :=    GetParamInt32(r,  "limit")

    reqHeight := bitcoin.ReqHeight(coin)

    payments := bitcoin.LoadSpendablePaymentsByAmount(mpkId, coin, min, max, reqHeight, uint(limit))

    ReturnJSON(API_OK, payments)
}
