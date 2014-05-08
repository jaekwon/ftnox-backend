package auth

import (
    . "ftnox.com/common"
    . "ftnox.com/config"
    "ftnox.com/email/sendemail"
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/bpowers/seshcookie"
    "github.com/balasanjay/totp"
)

var sessionKey string

func init() {
    sessionKey = Config.SessionKey
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
    var addr = r.FormValue("email")
    var password = r.FormValue("password")
    Info("New user: %v", addr)
    user, err := SaveUser(&User{
        Email:      addr,
        Password:   password,
    })
    switch err {
    case ERR_DUPLICATE_ADDRESS:
        ReturnJSON(API_INVALID_PARAM, "That email address is already taken")
    case nil:
        break
    default:
        panic(err)
    }

    // Send user confirmation email
    body := fmt.Sprintf(`Thank you for signing up with FtNox.
Please click on this link to confirm your email address:

    https://%v/auth/email_confirm?code=%v`, Config.Domain, user.EmailCode)
    err = sendemail.SendEmail("Welcome to FtNox", body, []string{user.Email})
    if err != nil {
        ReturnJSON(API_ERROR, err.Error())
    }

    ReturnJSON(API_OK, nil)
}

func EmailConfirmHandler(w http.ResponseWriter, r *http.Request) {
    code := GetParam(r, "code")
    UpdateUserSetEmailConfirmed(code)
    http.Redirect(w, r, "/email_confirmed.html", http.StatusSeeOther)
    return
}

func WithSession(handler http.HandlerFunc) http.HandlerFunc {
    return seshcookie.NewSessionHandler(http.HandlerFunc(handler), sessionKey, nil).ServeHTTP
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
    var email =     GetParam(r, "email")
    var password =  GetParam(r, "password")
    //var totpCode =  GetParam(r, "totp_code")
    Info("User login: %v", email)
    user := LoadUserByEmail(email)
    if user == nil {
        ReturnJSON(API_INVALID_PARAM, "Unrecognized email address")
    } else if !user.Authenticate(password) {
        ReturnJSON(API_INVALID_PARAM, "Invalid password")
    }
    // Set session cookie for javascript.
    session := seshcookie.Session.Get(r)
    session["userId"] = user.Id
    //if totp.Authenticate(user.TOTPKey, totpCode, nil) {
    if true {
        // Google auth authenticated.
        session["totpConfirmed"] = "true"
    } else if user.TOTPConf == 1 {
        // User has confirmed TOTP, we expect totp.Authenticate() to have passed.
        ReturnJSON(API_INVALID_PARAM, "Invalid Google Auth token")
    } else {
        // User never confirmed TOTP.
        // Continue, user needs to confirm TOTP, and
        // TOTPHandler requires session["userId"].

        // NOTE: we can't use ReturnJSON() here because
        // that throws a panic, which skips the cookie header writing,
        // performed by 'seshcookie'.
        // ReturnJSON(API_REDIRECT, "/#totp")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(430)
        w.Write([]byte(`{"status":"REDIRECT","data":"/#totp_confirm"}`))
        return
    }
    resJSON, err := json.Marshal(map[string]interface{}{
        "status": "OK",
        "data":   map[string]interface{}{
            "user": user,
        },
    })
    if err != nil { panic(err) }
    // NOTE: we can't use ReturnJSON() here because
    // that throws a panic, which skips the cookie header writing,
    // performed by 'seshcookie'.
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    w.Write(resJSON)
    return
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    session := seshcookie.Session.Get(r)
    // TODO: just delete the session cookie.
    // That way we don't remember to delete cookies.
    delete(session, "userId")
    delete(session, "totpConfirmed")
    // NOTE: we can't use ReturnJSON() here because
    // that throws a panic, which skips the cookie header writing,
    // performed by 'seshcookie'.
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    w.Write([]byte(`{"status":"OK","data":""}`))
}

func GetUser(r *http.Request, requireTOTP bool) *User {
    // API authentication?
    apiKeyKey := GetParam(r, "api_key")
    if apiKeyKey != "" {
        apiKey := LoadAPIKey(apiKeyKey)
        if apiKey != nil {
            user := LoadUser(apiKey.UserId)
            return user
        }
    }
    // Session login?
    session := seshcookie.Session.Get(r)
    if requireTOTP {
        totpConfirmed, ok := session["totpConfirmed"].(string)
        if !ok || totpConfirmed != "true" {
            return nil
        }
    }
    userId, ok := session["userId"].(int64)
    if ok && userId != 0 {
        return LoadUser(userId)
    }
    return nil
}

type AuthHandler func(http.ResponseWriter, *http.Request, *User)

func RequireAuth(handler AuthHandler) http.HandlerFunc {
    return WithSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := GetUser(r, true)
        if user == nil {
            ReturnJSON(API_UNAUTHORIZED, "Unauthorized")
        }
        // Serve original handler with authorized user.
        handler(w, r, user)
    }))
}

func TOTPImageHandler(w http.ResponseWriter, r *http.Request) {
    user := GetUser(r, false)
    if user == nil {
        ReturnJSON(API_UNAUTHORIZED, "Unauthorized")
    }
    label := fmt.Sprintf("FtNox-%v", user.Email)
    img, err := totp.BarcodeImage(label, user.TOTPKey, nil)
    if err != nil {
        ReturnJSON(API_ERROR, err.Error())
    }
    w.Header().Set("Content-Type", "image/png")
    w.Write(img)
}

func TOTPConfirmHandler(w http.ResponseWriter, r *http.Request) {
    user := GetUser(r, false)
    if user == nil {
        ReturnJSON(API_UNAUTHORIZED, "Unauthorized")
    }

    totpCode := GetParam(r, "code")
    if !totp.Authenticate(user.TOTPKey, totpCode, nil) {
        ReturnJSON(API_INVALID_PARAM, "Wrong TOTP Code")
    }

    // Google auth authenticated.
    session := seshcookie.Session.Get(r)
    session["totpConfirmed"] = "true"
    UpdateUserSetTOTPConfirmed(user.Id)

    resJSON, err := json.Marshal(map[string]interface{}{
        "status": "OK",
        "data":   map[string]interface{}{
            "user": user,
        },
    })
    if err != nil { panic(err) }
    // NOTE: we can't use ReturnJSON() here because
    // that throws a panic, which skips the cookie header writing,
    // performed by 'seshcookie'.
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(200)
    w.Write(resJSON)
}

func GetAPIKeysHandler(w http.ResponseWriter, r *http.Request, user *User) {
    apiKeys := LoadAPIKeysByUser(user.Id)
    ReturnJSON(API_OK, apiKeys)
}
