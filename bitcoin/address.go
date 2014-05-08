/*
    Convention: The more public methods are on top, but some of the useful private methods
    are also exposed.
*/

package bitcoin

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "github.com/conformal/btcec"
    "github.com/conformal/btcutil"
    "github.com/conformal/btcwire"
    "fmt"
    "errors"
    "strconv"
    "strings"
    "math/big"
    "hash"
    "crypto/hmac"
    "crypto/sha256"
    "crypto/sha512"
    "code.google.com/p/go.crypto/ripemd160"
    "crypto/ecdsa"
    "encoding/hex"
    "encoding/binary"
    "encoding/base64"
)

const (
    // BIP32 chainpath prefix
    CHAINPATH_PREFIX_DEPOSIT    = 0
    CHAINPATH_PREFIX_CHANGE     = 1
    CHAINPATH_PREFIX_SWEEP      = 2
    CHAINPATH_PREFIX_SWEEP_DRY  = 102
)

func ComputeAddress(coin string, pubKeyHex string, chainHex string, path string, index int32) string {
    pubKeyBytes := DerivePublicKeyForPath(
        HexDecode(pubKeyHex),
        HexDecode(chainHex),
        fmt.Sprintf("%v/%v", path, index),
    )
    return AddrFromPubKeyBytes(coin, pubKeyBytes)
}

func ComputePrivateKey(mprivHex string, chainHex string, path string, index int32) string {
    privKeyBytes := DerivePrivateKeyForPath(
        HexDecode(mprivHex),
        HexDecode(chainHex),
        fmt.Sprintf("%v/%v", path, index),
    )
    return HexEncode(privKeyBytes)
}

func ComputeAddressForPrivKey(coin string, privKey string) string {
    pubKeyBytes := PubKeyBytesFromPrivKeyBytes(HexDecode(privKey), true)
    return AddrFromPubKeyBytes(coin, pubKeyBytes)
}

func SignMessage(privKey string, message string, compress bool) string {
    prefixBytes := []byte("Bitcoin Signed Message:\n")
    messageBytes := []byte(message)
    bytes := []byte{}
    bytes =  append(bytes, byte(len(prefixBytes)))
    bytes =  append(bytes, prefixBytes...)
    bytes =  append(bytes, byte(len(messageBytes)))
    bytes =  append(bytes, messageBytes...)
    privKeyBytes := HexDecode(privKey)
    x, y := btcec.S256().ScalarBaseMult(privKeyBytes)
    ecdsaPubKey := ecdsa.PublicKey{
        Curve: btcec.S256(),
        X:     x,
        Y:     y,
    }
    ecdsaPrivKey := &ecdsa.PrivateKey{
        PublicKey: ecdsaPubKey,
        D:         new(big.Int).SetBytes(privKeyBytes),
    }
    sigbytes, err := btcec.SignCompact(btcec.S256(), ecdsaPrivKey, btcwire.DoubleSha256(bytes), compress)
    if err != nil { panic(err) }
    return base64.StdEncoding.EncodeToString(sigbytes)
}

// returns MPK, Chain, and master secret in hex.
func ComputeMastersFromSeed(seed string) (string, string, string) {
    secret, chain := I64([]byte("Bitcoin seed"), []byte(seed))
    pubKeyBytes := PubKeyBytesFromPrivKeyBytes(secret, true)
    return HexEncode(pubKeyBytes), HexEncode(chain), HexEncode(secret)
}

func ComputeWIF(coin string, privKey string, compress bool) string {
    return WIFFromPrivKeyBytes(coin, HexDecode(privKey), compress)
}

func ComputeTxId(rawTxHex string) string {
    return HexEncode(ReverseBytes(CalcHash256(HexDecode(rawTxHex))))
}

// Private methods...

func printKeyInfo(privKeyBytes []byte, pubKeyBytes []byte, chain []byte) {
    if pubKeyBytes == nil {
        pubKeyBytes = PubKeyBytesFromPrivKeyBytes(privKeyBytes, true)
    }
    addr := AddrFromPubKeyBytes("BTC", pubKeyBytes)
    Info("\nprikey:\t%v\npubKeyBytes:\t%v\naddr:\t%v\nchain:\t%v",
        HexEncode(privKeyBytes),
        HexEncode(pubKeyBytes),
        addr,
        HexEncode(chain))
}

func DerivePrivateKeyForPath(privKeyBytes []byte, chain []byte, path string) ([]byte) {
    data := privKeyBytes
    parts := strings.Split(path, "/")
    for _, part := range parts {
        prime := part[len(part)-1:] == "'"
        // prime == private derivation. Otherwise public.
        if prime { part = part[:len(part)-1] }
        i, err := strconv.Atoi(part)
        if err != nil { panic(err) }
        if i < 0 { panic(errors.New("index too large.")) }
        data, chain = DerivePrivateKey(data, chain, uint32(i), prime)
        //printKeyInfo(data, nil, chain)
    }
    return data
}

func DerivePublicKeyForPath(pubKeyBytes []byte, chain []byte, path string) ([]byte) {
    data := pubKeyBytes
    parts := strings.Split(path, "/")
    for _, part := range parts {
        prime := part[len(part)-1:] == "'"
        if prime { errors.New("cannot do a prime derivation from public key") }
        i, err := strconv.Atoi(part)
        if err != nil { panic(err) }
        if i < 0 { panic(errors.New("index too large.")) }
        data, chain = DerivePublicKey(data, chain, uint32(i))
        //printKeyInfo(nil, data, chain)
    }
    return data
}

func DerivePrivateKey(privKeyBytes []byte, chain []byte, i uint32, prime bool) ([]byte, []byte) {
    data := []byte{}
    if prime {
        i = i | 0x80000000
        data = append([]byte{byte(0)}, privKeyBytes...)
    } else {
        public := PubKeyBytesFromPrivKeyBytes(privKeyBytes, true)
        data = public
    }
    data = append(data, uint32ToBytes(i)...)
    data2, chain2 := I64(chain, data)
    return addScalars(privKeyBytes, data2), chain2
}

func DerivePublicKey(pubKeyBytes []byte, chain []byte, i uint32) ([]byte, []byte) {
    data := []byte{}
    data = append(data, pubKeyBytes...)
    data = append(data, uint32ToBytes(i)...)
    data2, chain2 := I64(chain, data)
    data2p := PubKeyBytesFromPrivKeyBytes(data2, true)
    return addPoints(pubKeyBytes, data2p), chain2
}

func addPoints(a []byte, b []byte) []byte {
    ap, err := btcec.ParsePubKey(a, btcec.S256())
    if err != nil { panic(err) }
    bp, err := btcec.ParsePubKey(b, btcec.S256())
    if err != nil { panic(err) }
    sumX, sumY := btcec.S256().Add(ap.X, ap.Y, bp.X, bp.Y)
    sum := (*btcec.PublicKey)(&ecdsa.PublicKey{
        Curve: btcec.S256(),
        X:     sumX,
        Y:     sumY,
    })
    return sum.SerializeCompressed()
}

func addScalars(a []byte, b []byte) []byte {
    aInt := new (big.Int).SetBytes(a)
    bInt := new (big.Int).SetBytes(b)
    sInt := new (big.Int).Add(aInt, bInt)
    return sInt.Mod(sInt, btcec.S256().N).Bytes()
}

func uint32ToBytes(i uint32) []byte {
    b := [4]byte{}
    binary.BigEndian.PutUint32(b[:], i)
    return b[:]
}

func HexEncode(b []byte) string {
    return hex.EncodeToString(b)
}

func HexDecode(str string) []byte {
    b, _ := hex.DecodeString(str)
    return b
}

func I64(key []byte, data []byte) ([]byte, []byte) {
    mac := hmac.New(sha512.New, key)
    mac.Write(data)
    I := mac.Sum(nil)
    return I[:32], I[32:]
}

func AddrFromPubKeyBytes(coin string, pubKeyBytes []byte) string {
    prefix := Config.GetCoin(coin).AddrPrefix
    h160 := CalcHash160(pubKeyBytes)
    h160 = append([]byte{prefix}, h160...)
    checksum := CalcHash256(h160)
    b := append(h160, checksum[:4]...)
    return btcutil.Base58Encode(b)
}

func WIFFromPrivKeyBytes(coin string, privKeyBytes []byte, compress bool) string {
    prefix := Config.GetCoin(coin).WIFPrefix
    bytes := append([]byte{prefix}, privKeyBytes...)
    if compress {
        bytes = append(bytes, byte(1))
    }
    checksum := CalcHash256(bytes)
    bytes = append(bytes, checksum[:4]...)
    return btcutil.Base58Encode(bytes)
}

func PubKeyBytesFromPrivKeyBytes(privKeyBytes []byte, compress bool) (pubKeyBytes []byte) {
    x, y := btcec.S256().ScalarBaseMult(privKeyBytes)
    pub := (*btcec.PublicKey)(&ecdsa.PublicKey{
        Curve: btcec.S256(),
        X:     x,
        Y:     y,
    })

    if compress {
        return pub.SerializeCompressed()
    }
    return pub.SerializeUncompressed()
}

// Calculate the hash of hasher over buf.
func CalcHash(buf []byte, hasher hash.Hash) []byte {
    hasher.Write(buf)
    return hasher.Sum(nil)
}

// calculate hash160 which is ripemd160(sha256(data))
func CalcHash160(buf []byte) []byte {
    return CalcHash(CalcHash(buf, sha256.New()), ripemd160.New())
}

// calculate hash256 which is sha256(sha256(data))
func CalcHash256(buf []byte) []byte {
    return CalcHash(CalcHash(buf, sha256.New()), sha256.New())
}

// calculate sha512(data)
func CalcSha512(buf []byte) []byte {
    return CalcHash(buf, sha512.New())
}

func ReverseBytes(buf []byte) []byte {
    res := []byte{}
    for i:=len(buf)-1; i>=0; i-- {
        res = append(res, buf[i])
    }
    return res
}
