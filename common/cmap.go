package common

import "sync"

type CMap struct {
    m   map[string]interface{}
    l   sync.Mutex
}

func NewCMap() *CMap {
    cmap := &CMap{}
    cmap.m = map[string]interface{}{}
    return cmap
}

func (cmap *CMap) Set(key string, value interface{}) {
    cmap.l.Lock()
    cmap.m[key] = value
    cmap.l.Unlock()
}

func (cmap *CMap) Get(key string) interface{} {
    cmap.l.Lock()
    value := cmap.m[key]
    cmap.l.Unlock()
    return value
}

func (cmap *CMap) Has(key string) bool {
    cmap.l.Lock()
    _, ok := cmap.m[key]
    cmap.l.Unlock()
    return ok
}

func (cmap *CMap) Delete(key string) {
    cmap.l.Lock()
    delete(cmap.m, key)
    cmap.l.Unlock()
}

func (cmap *CMap) Clear() {
    cmap.l.Lock()
    cmap.m = map[string]interface{}{}
    cmap.l.Unlock()
}
