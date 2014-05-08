package auth

import (
    . "ftnox.com/common"
    "testing"
)

func TestPersistUserDuplicateEmail(t *testing.T) {
    var u User
    u.Email = "someemail"+RandId(12)+"@fakehost.com"
    _, err := SaveUser(&u)
    if err != nil {
        t.Fatal(err)
    }

    uLoaded := LoadUserByEmail(u.Email)
    if uLoaded.Id != u.Id {
        t.Fatal("Loaded user's id did not match")
    }

    u.Id = 0
    _, err = SaveUser(&u)
    if err != ERR_DUPLICATE_ADDRESS {
        t.Fatal("duplicate email error not returned")
    }
}
