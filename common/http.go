package common

import (
    "net/http"
    "io/ioutil"
    "encoding/json"
)

func HttpGet(url string) (string, error) {
    resp, err := http.Get(url)
    if err != nil { return "", err }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return "", err }
    return string(body), nil
}

func HttpGetJSON(url string) (map[string]interface{}, error) {
    body, err := HttpGet(url)
    if err != nil { return nil, err }
    parsed := map[string]interface{}{}
    err = json.Unmarshal([]byte(body), &parsed)
    if err != nil { return nil, err }
    return parsed, nil
}
