package media_endpoints

import (
    "context"
    "net/http"
    "time"
    "errors"
    "strconv"
    "github.com/sirupsen/logrus"
    "github.com/gorilla/mux"
    "github.com/mongodb/mongo-go-driver/mongo"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "github.com/mongodb/mongo-go-driver/bson"
    "TwitterClone/wrappers"
    "TwitterClone/media"
    "TwitterClone/memcached"
)

var Log *logrus.Logger
var useCache = false
func GetMediaHandler(w http.ResponseWriter, r *http.Request) {
  start := time.Now()
    vars := mux.Vars(r)
    id := vars["id"]
    Log.Debug(id)
    var m media.Media
    oid, err := objectid.FromHex(id)
    if err != nil {
        Log.Error(err)
        // Not handling error for now.
    } else {
        var cacheErr error
        if useCache {
            cacheErr = memcached.Get(media.CacheKey(id), &m)
            if cacheErr != nil {
                Log.Info(cacheErr) // Probably a cache miss.
            }
        }
        if !useCache || cacheErr != nil {
            // Get from Mongo
            m, err = getMediaFromMongo(oid)
            if err != nil {
                Log.Error(err)
            }
        }
    }
    elapsed := time.Since(start)
    Log.WithFields(logrus.Fields{"timeElapsed":elapsed.String()}).Info("Get Media time elapsed")
    encodeResponse(w, m)
}

func getMediaFromMongo(oid objectid.ObjectID) (media.Media, error) {
  //dbStart := time.Now()
    var nilMedia media.Media
    client, err := wrappers.NewClient()
    if err != nil {
        return nilMedia, err
    }
    db := client.Database("twitter")
    coll := db.Collection("media")
    var m media.Media
    filter := bson.NewDocument(bson.EC.ObjectID("_id", oid))
    projection, err := mongo.Opt.Projection(bson.NewDocument(
        bson.EC.Int32("content", 1),
        bson.EC.Int32("contentType", 1),
    ))
    if err != nil {
        return nilMedia, err
    }
    err = coll.FindOne(context.Background(), filter, projection).Decode(&m)
    if err == nil && useCache { // Cache
        cacheErr := memcached.Set(media.CacheKey(oid.Hex()), &m)
        if cacheErr != nil {
            Log.Error(cacheErr)
        }
    }
    //elapsed := time.Since(dbStart)
    //Log.WithFields(logrus.Fields{"timeElapsed":elapsed.String()}).Info("Get Media time elapsed")
    return m, err
}

func encodeResponse(w http.ResponseWriter, m media.Media) {
    if (m.Content == nil) {
        err := errors.New("Media not found.")
        Log.Debug(err)
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", m.ContentType)
    w.Header().Set("Content-Length", strconv.Itoa(len(m.Content)))
    w.Write(m.Content)
}
