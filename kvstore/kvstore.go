package kvstore

import (
    . "ftnox.com/common"
    "ftnox.com/db"
    "ftnox.com/auth"
    "database/sql"
    "net/http"
    "fmt"
)

func SetForUser(user *auth.User, key, value string) {
    Set(fmt.Sprintf("user/%v/%v", user.Id, key), value)
}

func Set(key, value string) {
    _, err := db.Exec(
        `INSERT INTO kvstore (key_, value)
         VALUES (?, ?)`,
        key, value,
    )
    switch db.GetErrorType(err) {
    case nil: return
    case db.ERR_DUPLICATE_ENTRY:
        // Update instead
        _, err := db.Exec(
            `UPDATE kvstore SET value=? WHERE key=?`,
            value, key,
        )
        if err != nil { panic(err) }
        return
    default: panic(err)
    }
}

func Get(key string) string {
    var value string
    err := db.QueryRow(
        `SELECT value
         FROM kvstore WHERE key_=?`,
        key,
    ).Scan(&value)
    switch db.GetErrorType(err) {
    case sql.ErrNoRows:
        return ""
    case nil:
        return value
    default:
        panic(err)
    }
}

type KeyValue struct {
    Key   string `db:"key_"`
    Value string `db:"value"`
}

func GetAll(prefix string) map[string]string {
    kvMap := map[string]string{}
    rows, err := db.Query(
        `SELECT key_, value
         FROM kvstore WHERE key_ like ?`,
        prefix+"%",
    )
    if err != nil { panic(err) }
    for rows.Next() {
        var kvRow KeyValue
        err := rows.Scan(&kvRow)
        if err != nil { panic(err) }
        kvMap[kvRow.Key[len(prefix):]] = kvRow.Value
    }
    return kvMap
}

func GetHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    kvMap := GetAll(fmt.Sprintf("user/%v/", user.Id))
    ReturnJSON(API_OK, kvMap)
}

func SetHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    var key = r.FormValue("key")
    var value = r.FormValue("value")
    SetForUser(user, key, value)
    ReturnJSON(API_OK, nil)
}
