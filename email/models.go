package email

import (
    "ftnox.com/db"
)

// NOT USED YET
type OutgoingEmail struct {
    MessageID   string  `db:"message_id"`
    From        string  `db:"from_"`
    To          string  `db:"to_"`
    Subject     string  `db:"subject"`
    BodyPlain   string  `db:"body_plain"`

    TimeSent    uint64  `db:"time_sent"`
    Error       string  `db:"error"`
}

var OutgoingEmailModel = db.GetModelInfo(new(OutgoingEmail))

func SaveOutgoingEmail(email *OutgoingEmail) (*OutgoingEmail) {
    // TODO
    return nil
}
