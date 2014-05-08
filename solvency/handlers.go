package solvency

import (
    . "ftnox.com/common"
    "ftnox.com/auth"
    "net/http"
    "fmt"
    "os"
)

func LiabilitiesRootHandler(w http.ResponseWriter, r *http.Request) {
    coin := GetParamRegexp(r, "coin", RE_COIN, true)
    path := fmt.Sprintf("%v/.ftnox.com/solvency/%v/root.json", os.Getenv("HOME"), coin)
	http.ServeFile(w, r, path)
}

func LiabilitiesPartialHandler(w http.ResponseWriter, r *http.Request, user *auth.User) {
    coin := GetParamRegexp(r, "coin", RE_COIN, true)
    path := fmt.Sprintf("%v/.ftnox.com/solvency/%v/partial_trees/%v.json", os.Getenv("HOME"), coin, user.Id)
	http.ServeFile(w, r, path)
}

func AssetsHandler(w http.ResponseWriter, r *http.Request) {
    coin := GetParamRegexp(r, "coin", RE_COIN, true)
    path := fmt.Sprintf("%v/.ftnox.com/solvency/%v/assets.json", os.Getenv("HOME"), coin)
	http.ServeFile(w, r, path)
}
