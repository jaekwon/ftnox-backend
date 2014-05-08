// +build scrap

package main

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/db"
    "ftnox.com/auth"
    "ftnox.com/bitcoin"
    "ftnox.com/treasury"
    "ftnox.com/account"
    "ftnox.com/exchange"
    "encoding/hex"
    "encoding/json"
    "io/ioutil"
    "fmt"
    "math/rand"
    "time"
    "net/http"
    "net/url"
)

func makeMasters() {
    pubKey, chain, mPrivKey := bitcoin.ComputeMastersFromSeed("this is a test")
    Info(pubKey, ", ", chain, ", ", mPrivKey)

    // Now create an address for path "0/1/1"
    privKey := bitcoin.ComputePrivateKey(mPrivKey, chain, "0/1", 1)
    addr := bitcoin.ComputeAddressForPrivKey("BTC", privKey)

    // See that the addr matches the other derivation
    addr2 := bitcoin.ComputeAddress("BTC", pubKey, chain, "0/1", 1)
    if addr == addr2 {
        Info("Address matches: %v", addr)
    } else {
        Warn("Address does not match: %v vs %v", addr, addr2)
    }
}

func makeTreasury() {
    treasury.StorePrivateKeyForMPKPubKey("0271b114194fd037a410366b693b338ef7d190c8c2a20ce6164f0c9bc40df417d9", "8b8e225c197e9606e04ae9f0a7582e43177934b7383efbc452055abfbf3a5d0e")

    // Get the main wallet for testing.
    user := auth.LoadUserByEmail("system@ftnox.com")

    // Create a withdrawal request.
    account.AddWithdrawal(user.Id, "12uPSJQ9j3cHPwt2k2JgfkQ4MULwf5cxJH", "BTC", 20000)

    // Process the withdrawal request.
    treasury.Process("BTC")
}

func makeWIF() {
    privKey := "0C28FCA386C7A227600B2FE50B7CAE11EC86D3BF1FBE471BE89827E19D72AA1D"
    wif := bitcoin.ComputeWIF("BTC", privKey, true)
    Info("WIF: %v", wif)

    addr := bitcoin.ComputeAddressForPrivKey("BTC", privKey)
    Info("addr: %v", addr)

    pubKey := bitcoin.PubKeyBytesFromPrivKeyBytes(hexDecode(privKey), false)
    Info("pubKey: %v", hexEncode(pubKey))

    addr = bitcoin.AddrFromPubKeyBytes("BTC", pubKey)
    Info("addr2: %v", addr)

    message := "this is a message"
    signature := bitcoin.SignMessage(privKey, message, true)
    Info("signature: %v", signature)
}

func makeHash() {
    rawtx := "0100000001058f9896490e89664f599fb0e89a17da744749df316a0b7bbceb1169e2fb5879010000006a473044022064795e01493db0e09d0795c5f74d4e5b0e13889704cbcb5e49bd24343cfeecf80220208882fba54276cab98963f4667a7b19b468054b95f9ff828fa2a95abec7b694012102505ca2ceb2157aaf6205eb4b546833acd7308a80750a73229256b8d2f7c16e0cffffffff0200a014e3322600001976a9140d4040b14280779b6a084751d581a305b9046e0488ac00204aa9d10100001976a91414e07c0ead3d436f904c18ff72c1ef862dbf17d488ac00000000"
    txId := bitcoin.ComputeTxId(rawtx)
    Info("txid: %v", txId)
}

func makeJSON() {
    s, _ := (json.Marshal([]uint64{100}))
    Info(string(s))
}

func generateRandomUsers() []*auth.User {
    Info("Inserting random users")
    users := []*auth.User{}
    for i:=0; i<1000; i++ {
        email := fmt.Sprintf("test+%v@ftnox.com", i)
        user := auth.LoadUserByEmail(email)
        if user == nil {
            user = &auth.User{
                Email:  email,
                Scrypt: []byte{0},
                Salt:   []byte{0},
            }
            _, err := auth.SaveUser(user)
            if err != nil { panic(err) }
        }
        users = append(users, user)
    }
    Info("Done inserting random users")
    return users
}

func injectMoneyForUsers(users []*auth.User, coins []string) {
    Info("Injecting money for each user")
    for _, user := range users {
        for _, coin := range coins {
            err := db.DoBeginSerializable(func(tx *db.ModelTx) {
                account.UpdateBalanceByWallet(tx, user.Id, account.WALLET_MAIN, coin, int64(100000000000000), false)
            })
            if err != nil { panic(err) }
        }
    }
    Info("Done injecting money for each user")
}

func api(path string, params map[string]string) map[string]interface{} {
    v := url.Values{}
    for key, value := range params {
        v.Add(key, value)
    }
    resp, err := http.PostForm(fmt.Sprintf("http://localhost:%v/%v", Config.HTTPPort, path), v)
    if err != nil { panic(err) }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { panic(err) }
    var res = map[string]interface{}{}
    err = json.Unmarshal(body, &res)
    if err != nil {
        Warn("Body: %v", string(body))
        panic(err)
    }
    return res
}

func makeTrade() {
    users := generateRandomUsers()
    injectMoneyForUsers(users, []string{"BTC", "DOGE"})
    // Trade that money around
    last := time.Now().Unix()
    Info("Trading money around")
    for i:=0; i<1000000000; i++ {
        if i % 1000 == 0 {
            now := time.Now().Unix()
            diff := now - last
            Info("time for 1000 more moves: %v (%v)", diff, i)
            last = now
        }
        user1 := rand.Intn(1000)
        orderPrice := (rand.NormFloat64() + 10.0) / 10.0
        orderType := exchange.ORDER_TYPE_BID
        if rand.Int()%2 == 0 { orderType = exchange.ORDER_TYPE_ASK }
        var amount, basisAmount uint64
        if orderType == exchange.ORDER_TYPE_BID {
            //amount, basisAmount = 0, 100000
            amount, basisAmount = 1000000000, 0
        } else {
            amount, basisAmount = 1000000000, 0
        }

        res := api("exchange/add_order", map[string]string{
                "user_id":      fmt.Sprintf("%v", users[user1].Id),
                "api_key":      users[user1].APIKey,
                "order_type":   orderType,
                "market":       "DOGE/BTC",
                "amount":       fmt.Sprintf("%v", amount),
                "basis_amount": fmt.Sprintf("%v", basisAmount),
                "price":        fmt.Sprintf("%v", orderPrice),
            },
        )
        if res["status"] != "OK" {
            panic(NewError("Trade API response wasn't OK: %v", res))
        }
        /*
        order := &exchange.Order{
            Type:           orderType,
            UserId:         users[user1].Id,
            Coin:           "DOGE",
            Amount:         uint64(float64(100.0)/float32(orderPrice)),
            BasisCoin:      "BTC",
            BasisAmount:    100,
            Price:          orderPrice,
        }
        //Info("Added order for %v", user1)
        exchange.AddOrder(order)
        exchange.ProcessNextOrder()
        */

        time.Sleep(1 * time.Second)
    }
    Info("Done trading money around")
}

func makeRPCCall() {
    ret, err := bitcoin.SendRPC("BTC", "createrawtransaction", []interface{}{map[string]interface{}{"txid":"01b8d6b1b3e1dc175253f81269dc0753767f9ee4ef31ad62f2e7c453925391fa", "vout":0}}, map[string]interface{}{"1E9jy9377qyUCjHYTeo3XPRDDLeJBku8cU":0.005, "1MjejBbkTxgEbQZkNX2pKGqMkpNZRDk7Ec":0.003})
    Info(">> %v", ret)
    Info(">> %v", err)
}

func main() {
    //makeMasters()
    //makeWIF()
    //makeHash()
    //makeJSON()
    //makeTreasury()
    makeTrade()
    //makeRPCCall()
}

func hexEncode(b []byte) string {
    return hex.EncodeToString(b)
}

func hexDecode(str string) []byte {
    b, _ := hex.DecodeString(str)
    return b
}
