// +build sign_message

package main

import (
    "ftnox.com/bitcoin"
    "io/ioutil"
    "fmt"
    "os"
    "github.com/howeyc/gopass"
)

func main() {

    fmt.Println("Enter secret seed:")
    seed := string(gopass.GetPasswd())

    pubKey, chain, privKey := bitcoin.ComputeMastersFromSeed(seed)
    fmt.Println("Derived pubkey: ", pubKey)

    fmt.Println("Enter message to sign, ctrl-d ctrl-d when done:")
    message, err := ioutil.ReadAll(os.Stdin)
    if err != nil { panic(err) }

    signature := bitcoin.SignMessage(privKey, string(message), true)
    fmt.Printf(`

-----BEGIN BITCOIN SIGNED MESSAGE-----
%v
-----BEGIN BITCOIN SIGNATURE-----
Version: Bitcoin-qt (1.0)
PublicKey: %v
Chain: %v

%v
-----END BITCOIN SIGNATURE-----

`, string(message), pubKey, chain, signature)

}
