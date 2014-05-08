package alert

import (
    . "ftnox.com/config"
    "ftnox.com/email/sendemail"
    "github.com/sfreiberg/gotwilio"
    "time"
    "fmt"
    "log"
)

// Master public key for generating account deposit addresses
var twilioSid string
var twilioToken string
var twilioFrom string
var twilioTo string
var twilioMinInterval int

func init() {
    twilioSid =   Config.TwilioSid
    twilioToken = Config.TwilioToken
    twilioFrom =  Config.TwilioFrom
    twilioTo =    Config.TwilioTo
    twilioMinInterval = Config.TwilioMinInterval
}

var last int64 = 0
var count int = 0

func Alert(message string) {
    log.Printf("<!> ALERT <!>\n"+message)
    now := time.Now().Unix()
    if now - last > int64(twilioMinInterval) {
        message = fmt.Sprintf("%v:%v", Config.AppName, message)
        if count > 0 {
            message = fmt.Sprintf("%v (+%v more since)", message, count)
            count = 0
        }
        go sendTwilio(message)
        go sendEmail(message)
    } else {
        count++
    }
}

func sendTwilio(message string) {
    defer func() {
        if err := recover(); err != nil { log.Printf("sendTwilio error: %v", err) }
    }()
    if len(message) > 50 { message = message[:50] }
    twilio := gotwilio.NewTwilioClient(twilioSid, twilioToken)
    res, exp, err := twilio.SendSMS(twilioFrom, twilioTo, message, "", "")
    if exp != nil || err != nil {
        log.Printf("sendTwilio error: %v %v %v", res, exp, err)
    }
}

func sendEmail(message string) {
    defer func() {
        if err := recover(); err != nil { log.Printf("sendEmail error: %v", err) }
    }()
    subject := message
    if len(subject) > 80 { subject = subject[:80] }
    err := sendemail.SendEmail(subject, message, []string{"errors@ftnox.com"})
    if err != nil {
        log.Printf("sendEmail error: %v\n%v", err, message)
    }
}
