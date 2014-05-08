package common

import (
    "strconv"
)

const (
    SATOSHI =  int64(100000000)
    USATOSHI = uint64(100000000)
)

func MaxUint64(a, b uint64) uint64 {
    if a > b { return a }
    return b
}

func MinUint64(a, b uint64) uint64 {
    if a < b { return a }
    return b
}

func MaxInt64(a, b int64) int64 {
    if a > b { return a }
    return b
}

func MinInt64(a, b int64) int64 {
    if a < b { return a }
    return b
}

func MaxUint32(a, b uint32) uint32 {
    if a > b { return a }
    return b
}

func MinUint32(a, b uint32) uint32 {
    if a < b { return a }
    return b
}

func MaxInt32(a, b int32) int32 {
    if a > b { return a }
    return b
}

func MinInt32(a, b int32) int32 {
    if a < b { return a }
    return b
}

func F64ToI64(f float64) (int64) {
    return RoundFloat64(f * 100000000.0)
}

func F64ToUI64(f float64) (uint64) {
    if f >= float64(0) {
        return uint64(RoundFloat64(f * 100000000.0))
    } else {
        panic("F64ToUI64 cannot cast negative float64 to uint64")
    }
}

func I64ToF64(i int64) float64 {
    return float64(i) / 100000000.0
}

func UI64ToF64(i uint64) float64 {
    return float64(i) / 100000000.0
}

func RoundFloat64(f float64) int64 {
    if f >= float64(0) {
        return int64(f + 0.5)
    } else {
        return int64(f - 0.5)
    }
}

func F64ToS(f float64, sig int) string {
    return strconv.FormatFloat(f, 'e', sig-1, 64)
}

func F64ToF(f float64, sig int) float64 {
    s := strconv.FormatFloat(f, 'e', sig-1, 64)
    f, _ = strconv.ParseFloat(s, 64)
    return f
}

func CompareF64(f1 float64, f2 float64, sig int) int {
    s1 := F64ToS(f1, sig)
    s2 := F64ToS(f2, sig)
    if s1 == s2 { return 0 }
    if f1 < f2 { return -1 }
    return 1
}
