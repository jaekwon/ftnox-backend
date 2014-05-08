package common

import (
    "testing"
)

func TestConversions(t *testing.T) {
    for i:=int64(-10000); i<20000; i++ {
        pf := I64ToF64(i)
        pi := F64ToI64(pf)
        if i != pi {
            t.Fatalf("Expected %v but got %v", i, pi)
        }
    }
}
