package memcached

import (
    "time"
    "encoding/json"
    "github.com/sirupsen/logrus"
    "github.com/bradfitz/gomemcache/memcache"
)
var Log *logrus.Logger
var mc *memcache.Client
const (
    MaxIdleConns = 100
)

func init() {
    mc = memcache.New("memcached-1:11211")
    mc.MaxIdleConns = MaxIdleConns
}

func Delete(key string) error {
   return mc.Delete(key)
}

func Get(key string, v interface{}) error {
    Log.SetLevel(logrus.InfoLevel)
    start := time.Now()
    item, err := mc.Get(key)
    elapsed := time.Since(start)
    Log.Info("Memcached lib GET " + key + " , elapsed: " +
        elapsed.String())
    if err != nil {
        return err
    } else {
        umStart := time.Now()
        err = json.Unmarshal(item.Value, v)
        elapsed = time.Since(umStart)
        Log.Info("Memcached lib GET unmarshal: " + key + " , elapsed: " +
            elapsed.String())
        return err
    }
}

func Set(key string, v interface{}) error {
    Log.SetLevel(logrus.InfoLevel)
    start := time.Now()
    b, err := json.Marshal(v)
    if err != nil {
        return err
    }
    var item memcache.Item
    item.Key = key
    item.Value = b
    err = mc.Set(&item)
    elapsed := time.Since(start)
    Log.Info("Memcached lib SET elapsed: " + elapsed.String())
    return err
}
