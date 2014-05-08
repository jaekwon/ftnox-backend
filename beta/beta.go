package beta

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "encoding/json"
    "net/http"
    "time"
    "io/ioutil"
)

// HANDLERS

func SignupHandler(w http.ResponseWriter, r *http.Request) {
    body, err := ioutil.ReadAll(r.Body)
    if err != nil { panic(err) }
    header, err := json.MarshalIndent(r.Header, "", "  ")
    if err != nil { panic(err) }

    signup := &BetaSignup{
        Body:       string(body),
        Header:     string(header),
    }

    SaveBetaSignup(signup)

    ReturnJSON(API_OK, nil)
}

// MODELS

type BetaSignup struct {
    Id          int64   `json:"id"          db:"id,autoinc"`
    Body        string  `json:"body"        db:"body"`
    Header      string  `json:"header"      db:"header"`
    Time        int64   `json:"time"        db:"time"`
}

var BetaSignupModel = db.GetModelInfo(new(BetaSignup))

func SaveBetaSignup(signup *BetaSignup) (*BetaSignup) {
    if signup.Time == 0 { signup.Time = time.Now().Unix() }
    _, err := db.Exec(
        `INSERT INTO beta_signup (`+BetaSignupModel.FieldsInsert+`)
         VALUES (`+BetaSignupModel.Placeholders+`)`,
        signup,
    )
    if err != nil { panic(err) }
    return signup
}
