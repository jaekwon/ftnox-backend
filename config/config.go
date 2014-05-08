// NOTE: Keep config a json thing so other language integration stays easy.

package config

import (
    bitcoin "ftnox.com/bitcoin/types"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
    "strings"
    "errors"
    "crypto/rand"
    "encoding/hex"
)

type ConfigType struct {
    AppName string

    Domain string
    HTTPPort int
    SessionKey string

    DbDriver string
    DbURL string
    DbDebugLog bool

    HotMPKPubKey string
    HotMPKChain string

    TwilioSid string
    TwilioToken string
    TwilioFrom string
    TwilioTo string
    TwilioMinInterval int

    GMailPassword string

    Coins []*bitcoin.Coin
}

func (cfg *ConfigType) validate() error {
    if cfg.AppName == ""           { cfg.AppName = "DEV" }
    if cfg.HTTPPort == 0           { return errors.New("HTTPPort must be set") }
    if cfg.DbDriver == ""          { return errors.New("DbDriver must be set") }
    if cfg.DbURL == ""             { return errors.New("DbURL must be set") }
    if cfg.SessionKey == ""        { return errors.New("SessionKey must be set") }
    if cfg.HotMPKPubKey == ""      { return errors.New("HotMPKPubKey must be set") }
    if cfg.HotMPKChain == ""       { return errors.New("HotMPKChain must be set") }
    if cfg.TwilioSid == ""         { return errors.New("TwilioSid must be set") }
    if cfg.TwilioToken == ""       { return errors.New("TwilioToken must be set") }
    if cfg.TwilioFrom == ""        { return errors.New("TwilioFrom must be set") }
    if cfg.TwilioTo == ""          { return errors.New("TwilioTo must be set") }
    if cfg.TwilioMinInterval == 0  { return errors.New("TwilioMinInterval must be set") }
    if cfg.GMailPassword == ""     { return errors.New("GMailPassword must be set") }
    if cfg.Domain == ""            { return errors.New("Domain must be set") }
    if len(cfg.Coins) == 0         { return errors.New("Coins must be set") }
    return nil
}

func (cfg *ConfigType) GetCoin(name string) *bitcoin.Coin {
    for _, coin := range cfg.Coins {
        if coin.Name == name { return coin }
    }
    return nil
}

var defaultConfig = `
{
    "Domain":           "dev.ftnox.com",
    "HTTPPort":         8888,
    "SessionKey":       "::SESSIONKEY::",

    "DbDriver":         "postgres",
    "DbURL":            "postgres://postgres@localhost/ftnox?sslmode=disable",
    "DbDebugLog":       false,

    "HotMPKPubKey":     "CHANGEME",
    "HotMPKChain":      "CHANGEME",

    "Coins": [
        {
            "Name":       "BTC",
            "Type":       "C",
            "ConfSec":    600,
            "RPCUser":    "bitcoinrpc",
            "RPCPass":    "CHANGEME",
            "RPCHost":    "CHANGEME",
            "TotConf":    6,
            "ReqConf":    3,
            "AddrPrefix": 0,
            "WIFPrefix":  128,
            "MinerFee":   20000,
            "MinTrade":   40000
        },
        {
            "Name":       "LTC",
            "Type":       "C",
            "ConfSec":    250,
            "RPCUser":    "litecoinrpc",
            "RPCPass":    "CHANGEME",
            "RPCHost":    "CHANGEME",
            "TotConf":    24,
            "ReqConf":    12,
            "AddrPrefix": 48,
            "WIFPrefix":  176,
            "MinerFee":   100000,
            "MinTrade":   200000
        },
        {
            "Name":       "USD",
            "Symbol":     "$",
            "Type":       "F",
            "MinTrade":   1000000
        }
    ],

    "TwilioSid":            "CHANGEME",
    "TwilioToken":          "CHANGEME",
    "TwilioFrom":           "+CHANGEME",
    "TwilioTo":             "+CHANGEME",
    "TwilioMinInterval":    600,

    "GMailPassword":        "CHANGEME"
}
`

var Config ConfigType

func init() {
    configFile := os.Getenv("HOME") + "/.ftnox.com/config.json"

    // try to read configuration. if missing, write default
    configBytes, err := ioutil.ReadFile(configFile)
    if err != nil {
        writeDefaultConfig(configFile)
        fmt.Println("Config file written to config.json. Please edit & run again")
        os.Exit(1)
        return
    }

    // try to parse configuration. on error, die
    Config = ConfigType{}
    err = json.Unmarshal(configBytes, &Config)
    if err != nil {
        log.Panicf("Invalid configuration file %s: %v", configFile, err)
    }
    err = Config.validate()
    if err != nil {
        log.Panicf("Invalid configuration file %s: %v", configFile, err)
    }
}

func generateSessionKey() string {
    bytes := &[30]byte{}
    rand.Read(bytes[:])
    return hex.EncodeToString(bytes[:])
}

func writeDefaultConfig(configFile string) {
    log.Printf("Creating default configration file %s", configFile)
    config := strings.Replace(defaultConfig, "::SESSIONKEY::", generateSessionKey(), -1)
    if strings.Index(configFile, "/") != -1 {
        err := os.MkdirAll(filepath.Dir(configFile), 0700)
        if err != nil { panic(err) }
    }
    err := ioutil.WriteFile(configFile, []byte(config), 0600)
    if err != nil {
        panic(err)
    }
}
