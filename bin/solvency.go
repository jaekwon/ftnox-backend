// +build solvency

package main

import (
    . "ftnox.com/common"
    "ftnox.com/bitcoin"
    "ftnox.com/db"
    "encoding/json"
    "strings"
    "path/filepath"
    "io/ioutil"
    "os"
    "os/exec"
    "fmt"
)

type UserBalance struct {
    User    string `json:"user"`
    Balance int64  `json:"balance"`
}

type AddressBalance struct {
    Address     string `json:"address"`
    MPKId       int64  `json:"-"`
    ChainPath   string `json:"chain_path"`
    Amount      uint64 `json:"amount"`
}

func main() {
    computeLiabilities()
    computeAssets()
}

func computeLiabilities() {
    allBalances := map[string]map[int64]int64{}

    Info("Computing Liabilities...")

    err := db.DoBegin("", func(tx *db.ModelTx) {
        _, err := tx.Exec("LOCK account_balance IN SHARE MODE;")
        if err != nil { panic(err) }
        rows, err := tx.Query(
            `SELECT user_id, wallet, coin, amount FROM account_balance
             WHERE wallet='main' OR wallet='reserved_o' OR wallet='reserved_w'
             ORDER BY user_id ASC`)
        if err != nil { panic(err) }
        for rows.Next() {
            var userId int64
            var wallet string
            var coin   string
            var amount int64
            err := rows.Scan(&userId, &wallet, &coin, &amount)
            if err != nil { panic(err) }
            if allBalances[coin] == nil { allBalances[coin] = map[int64]int64{} }
            allBalances[coin][userId] += amount
        }
    })
    if err != nil { panic(err) }

    for coin, balances := range allBalances {
        var userBalances = []UserBalance{}
        var sum int64
        for userId, amount := range balances {
            sum += amount
            userBalances = append(userBalances, UserBalance{fmt.Sprintf("%v", userId), amount})
        }
        fmt.Printf("Total %v:\t%v\n", coin, sum)
        dirPath := os.Getenv("HOME")+"/.ftnox.com/solvency/"+coin
        filePath := dirPath+"/accounts.json"
        writeToFile(filePath, userBalances)
        runBlproof(dirPath)
    }
}

func computeAssets() {
    Info("Computing Assets...")

    // TODO: filter results by blockheight.
    coinAddressBalances := map[string]map[string]*AddressBalance{}
    rows, err := db.Query(
        `SELECT p.coin, p.address, p.amount, p.mpk_id, a.chain_path, a.chain_idx FROM payment AS p
         INNER JOIN address AS a ON p.address=a.address
         WHERE p.spent=0 AND p.orphaned=0`)
    if err != nil { panic(err) }
    for rows.Next() {
        var coin, address string
        var amount uint64
        var mpkId int64
        var chainPath string
        var chainIdx int32
        err := rows.Scan(&coin, &address, &amount, &mpkId, &chainPath, &chainIdx)
        if err != nil { panic(err) }
        if coinAddressBalances[coin] == nil { coinAddressBalances[coin] = map[string]*AddressBalance{} }
        if coinAddressBalances[coin][address] == nil {
            coinAddressBalances[coin][address] = &AddressBalance{
                Address:    address,
                Amount:     amount,
                ChainPath:  fmt.Sprintf("%v/%v", chainPath, chainIdx),
                MPKId:      mpkId,
            }
        } else {
            coinAddressBalances[coin][address].Amount += amount
        }
    }

    for coin, addressBalances := range coinAddressBalances {
        // Group addresses by MPK.
        var mpkBalances = map[int64][]*AddressBalance{}
        var sum uint64
        for _, addrBalance := range addressBalances {
            sum += addrBalance.Amount
            mpkBalances[addrBalance.MPKId] = append(mpkBalances[addrBalance.MPKId], addrBalance)
            //fmt.Printf("[%v] %v\t%v\t%v\n", coin, address, addrBalance.ChainPath, addrBalance.Amount)
        }
        fmt.Printf("Total %v:\t%v\n", coin, sum)
        // Write to assets.json
        var mpkAssets = []interface{}{}
        for mpkId, addrBalances := range mpkBalances {
            mpk := bitcoin.LoadMPK(mpkId)
            mpkAsset := map[string]interface{}{
                "public_key":   mpk.PubKey,
                "chain":        mpk.Chain,
                "assets":       addrBalances,
            }
            mpkAssets = append(mpkAssets, mpkAsset)
        }
        dirPath := os.Getenv("HOME")+"/.ftnox.com/solvency/"+coin
        filePath := dirPath+"/assets.json"
        writeToFile(filePath, mpkAssets)
    }
}

func writeToFile(filePath string, obj interface{}) {
    b, err := json.Marshal(obj)
    if err != nil { panic(err) }

    if strings.Index(filePath, "/") != -1 {
        err = os.MkdirAll(filepath.Dir(filePath), 0700)
        if err != nil { panic(err) }
    }
    err = ioutil.WriteFile(filePath, b, 0600)
    if err != nil { panic(err) }
}

func runBlproof(dirPath string) {
    cmd := exec.Command("blproof", "generate", "-f", "accounts.json")
    cmd.Dir = dirPath
    err := cmd.Start()
    if err != nil { panic(err) }
    err = cmd.Wait()
    if err != nil { panic(err) }
}
