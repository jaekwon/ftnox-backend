package bitcoin

func IsValidDenom(denom string) bool {
    if denom == "BTC" || denom == "USD" {
        return true
    }
    return false
}
