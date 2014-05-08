package common

import (
    "fmt"
    "errors"
)

func NewError(fmtStr string, args ...interface{}) error {
    return errors.New(fmt.Sprintf(fmtStr, args...))
}
