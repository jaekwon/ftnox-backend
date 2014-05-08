// +build server

package main

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/auth"
    "ftnox.com/kvstore"
    "ftnox.com/account"
    "ftnox.com/exchange"
    "ftnox.com/treasury"
    "ftnox.com/beta"
    "ftnox.com/solvency"
    _ "ftnox.com/daemon"
    "strings"
    "net/http"
    "log"
    "fmt"
)

func main() {

    // Resources
    http.HandleFunc("/",                            StaticHandler)

    // Auth
    http.HandleFunc("/auth/register",               auth.RegisterHandler)
    http.HandleFunc("/auth/email_confirm",          auth.EmailConfirmHandler)
    http.HandleFunc("/auth/login",                  auth.WithSession(auth.LoginHandler))
    http.HandleFunc("/auth/logout",                 auth.WithSession(auth.LogoutHandler))
    http.HandleFunc("/auth/totp_qr.png",            auth.WithSession(auth.TOTPImageHandler))
    http.HandleFunc("/auth/totp_confirm",           auth.WithSession(auth.TOTPConfirmHandler))
    http.HandleFunc("/auth/api_keys",               auth.RequireAuth(auth.GetAPIKeysHandler))

    // KVStore
    http.HandleFunc("/kvstore/get",                 auth.RequireAuth(kvstore.GetHandler))
    http.HandleFunc("/kvstore/set",                 auth.RequireAuth(kvstore.SetHandler))

    // Account
    http.HandleFunc("/account/balance",             auth.RequireAuth(account.BalanceHandler))
    http.HandleFunc("/account/deposit_address",     auth.RequireAuth(account.DepositAddressHandler))
    http.HandleFunc("/account/deposits",            auth.RequireAuth(account.DepositsHandler))
    http.HandleFunc("/account/withdraw",            auth.RequireAuth(account.WithdrawHandler))
    http.HandleFunc("/account/withdrawals",         auth.RequireAuth(account.WithdrawalsHandler))

    // Solvency
    http.HandleFunc("/solvency/liabilities_root",   solvency.LiabilitiesRootHandler)
    http.HandleFunc("/solvency/liabilities_partial",auth.RequireAuth(solvency.LiabilitiesPartialHandler))
    http.HandleFunc("/solvency/assets",             solvency.AssetsHandler)

    // Exchange
    http.HandleFunc("/exchange/markets",            exchange.MarketsHandler)
    http.HandleFunc("/exchange/orderbook",          exchange.OrderBookHandler)
    http.HandleFunc("/exchange/pricelog",           exchange.PriceLogHandler)
    http.HandleFunc("/exchange/add_order",          auth.RequireAuth(exchange.AddOrderHandler))
    http.HandleFunc("/exchange/cancel_order",       auth.RequireAuth(exchange.CancelOrderHandler))
    http.HandleFunc("/exchange/pending_orders",     auth.RequireAuth(exchange.GetPendingOrdersHandler))
    http.HandleFunc("/exchange/trade_history",      auth.RequireAuth(exchange.TradeHistoryHandler))

    // Treasury
    http.HandleFunc("/treasury/",                   auth.RequireAuth(treasury.StaticHandler))
    http.HandleFunc("/treasury/mpk",                auth.RequireAuth(treasury.StorePrivateKeyHandler))
    http.HandleFunc("/treasury/withdrawals",        auth.RequireAuth(treasury.GetWithdrawalsHandler))
    http.HandleFunc("/treasury/resume_withdrawal",  auth.RequireAuth(treasury.ResumeWithdrawalHandler))
    http.HandleFunc("/treasury/deposits",           auth.RequireAuth(treasury.GetDepositsHandler))
    http.HandleFunc("/treasury/credit_user",        auth.RequireAuth(treasury.CreditUserHandler))

    // Beta signup
    http.HandleFunc("/beta",                        beta.SignupHandler)

    // Serve HTTP on localhost only. Let Nginx terminate HTTPS for us.
    address := fmt.Sprintf("127.0.0.1:%v", Config.HTTPPort)
    log.Printf("Listening on http://%s\n", address)
    log.Fatal(http.ListenAndServe(address, RecoverAndLogHandler(http.DefaultServeMux)))
}

//
// SERVE HTML, CSS, JS
//

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	var path string
	if strings.HasSuffix(r.URL.Path, "/") {
		path = r.URL.Path + "index.html"
	} else {
		path = r.URL.Path
	}
	http.ServeFile(w, r, "static/main/"+path)
}
