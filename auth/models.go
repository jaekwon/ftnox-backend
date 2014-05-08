package auth

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "code.google.com/p/go.crypto/scrypt"
    "database/sql"
    "errors"
    "bytes"
    "strings"
    "math"
)

var (
    ERR_DUPLICATE_ADDRESS = errors.New("ERR_DUPLICATE_ADDRESS")
)

// USER

type User struct {
    Id          int64  `json:"id"           db:"id,autoinc"`
    Email       string `json:"email"        db:"email"`
    EmailCode   string `json:"-"            db:"email_code"`
    EmailConf   int32  `json:"-"            db:"email_conf"`
    Password    string `json:"-"`
    Scrypt      []byte `json:"-"            db:"scrypt"`
    Salt        []byte `json:"-"            db:"salt"`
    TOTPKey     []byte `json:"-"            db:"totp_key"`
    TOTPConf    int32  `json:"totpConf"     db:"totp_conf"`
    ChainIdx    int32  `json:"-"            db:"chain_idx"`
    Roles       string `json:"roles"        db:"roles"`
}

var UserModel = db.GetModelInfo(new(User))

func (user *User) HasRole(role string) bool {
    roles := strings.Split(user.Roles, ",")
    for _, rl := range roles {
        if role == rl { return true }
    }
    return false
}

func (user *User) Authenticate(password string) bool {
    // Scrypt the password.
    scryptPassword, err := scrypt.Key([]byte(password), user.Salt, 16384, 8, 1, 32)
    if err != nil { panic(err) }
    return bytes.Equal(scryptPassword, user.Scrypt)
}

// Create a new user.
func SaveUser(user *User) (*User, error) {

    // Create email confirmation code.
    if user.EmailCode == "" { user.EmailCode = RandId(24) }
    // Create TOTPKey.
    if len(user.TOTPKey) == 0 { user.TOTPKey = RandBytes(10) }

    // Scrypt the password.
    if user.Password != "" {
        salt := RandId(12)
        scryptPass, err := scrypt.Key([]byte(user.Password), []byte(salt), 16384, 8, 1, 32)
        if err != nil { return nil, err }
        user.Salt = []byte(salt)
        user.Scrypt = scryptPass
    }

    err := db.DoBeginSerializable(func(tx *db.ModelTx) {
        // Insert into users table.
        err := tx.QueryRow(
            `INSERT INTO auth_user (`+UserModel.FieldsInsert+`)
             VALUES (`+UserModel.Placeholders+`)
             RETURNING id`,
            user,
        ).Scan(&user.Id)
        if err != nil { panic(err) }

        // Set the chain_idx
        if user.Id > math.MaxInt32 { panic("User autoinc id has exceeded MaxInt32") }
        user.ChainIdx = int32(user.Id)
        _, err = tx.Exec(
            `UPDATE auth_user
             SET chain_idx = id
             WHERE id=?`,
            user.Id,
        )
        if err != nil { panic(err) }

        // Generate an API key for the user
        apiKey := &APIKey{Key:RandId(24), UserId: user.Id}
        SaveAPIKey(tx, apiKey)

    })
    switch db.GetErrorType(err) {
    case db.ERR_DUPLICATE_ENTRY:
        return nil, ERR_DUPLICATE_ADDRESS
    case nil:
        break
    default:
        panic(err)
    }

    return user, nil
}

func UpdateUserSetEmailConfirmed(emailCode string) {
    _, err := db.Exec(
        `UPDATE auth_user
         SET email_conf=1
         WHERE email_code=?`,
        emailCode,
    )
    if err != nil { panic(err) }
}

func UpdateUserSetTOTPConfirmed(userId int64) {
    _, err := db.Exec(
        `UPDATE auth_user
         SET totp_conf=1
         WHERE id=?`,
        userId,
    )
    if err != nil { panic(err) }
}

func LoadUserByEmail(email string) *User {
    var user User
    err := db.QueryRow(
        `SELECT `+UserModel.FieldsSimple+`
         FROM auth_user WHERE email=?`,
        email,
    ).Scan(
        &user,
    )
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &user
    default:
        panic(err)
    }
}

func LoadUser(userId int64) *User {
    var user User
    err := db.QueryRow(
        `SELECT `+UserModel.FieldsSimple+`
         FROM auth_user WHERE id=?`,
        userId,
    ).Scan(
        &user,
    )
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &user
    default:
        panic(err)
    }
}

// API KEY

type APIKey struct {
    Key         string `json:"key"          db:"key"`
    UserId      int64  `json:"-"            db:"user_id"`
    Roles       string `json:"roles"        db:"roles"`
}

var APIKeyModel = db.GetModelInfo(new(APIKey))

func SaveAPIKey(tx *db.ModelTx, apiKey *APIKey) (*APIKey) {
    _, err := tx.Exec(
        `INSERT INTO auth_api_key (`+APIKeyModel.FieldsInsert+`)
         VALUES (`+APIKeyModel.Placeholders+`)`,
        apiKey,
    )
    if err != nil { panic(err) }
    return apiKey
}

func LoadAPIKey(key string) *APIKey {
    var apiKey APIKey
    err := db.QueryRow(
        `SELECT `+APIKeyModel.FieldsSimple+`
         FROM auth_api_key WHERE key=?`,
        key,
    ).Scan(
        &apiKey,
    )
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return nil
    case nil:
        return &apiKey
    default:
        panic(err)
    }
}

func LoadAPIKeysByUser(userId int64) []*APIKey {
    rows, err := db.QueryAll(APIKey{},
        `SELECT `+APIKeyModel.FieldsSimple+`
         FROM auth_api_key
         WHERE user_id=?`,
        userId,
    )
    if err != nil { panic(err) }
    return rows.([]*APIKey)
}
