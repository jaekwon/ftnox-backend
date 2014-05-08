// All results must be cryptographically secure.

package common

import (
    "math/big"
    "crypto/rand"
    "encoding/hex"
)

func RandBytes(numBytes int) []byte {
    bytes := make([]byte, numBytes)
    rand.Read(bytes[:])
    return bytes
}

func RandHex(numBytes int) string {
    return hex.EncodeToString(RandBytes(numBytes))
}

const allRandChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandId(numChars int) string {
    var res string
    for i:=0; i<numChars; i++ {
        v, err := rand.Int(rand.Reader, big.NewInt(int64(62)))
        if err != nil { panic(err) }
        randIndex := int(v.Int64())
        res = res + allRandChars[randIndex:randIndex+1]
    }
    return res
}
