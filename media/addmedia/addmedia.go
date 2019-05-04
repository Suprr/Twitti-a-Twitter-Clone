package addmedia

import (
    "context"
    "net/http"
    "time"
    "errors"
    "io"
    "bytes"
    "github.com/sirupsen/logrus"
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "TwitterClone/wrappers"
    "TwitterClone/media"
    "TwitterClone/memcached"
)

const(
    fileKey = "content"
    defaultMaxMemory = 32 << 20 // 32 MB
)

type response struct {
    Status string `json:"status"`
    ID string  `json:"id,omitempty"`
    Error string `json:"error,omitempty"`
}

var Log *logrus.Logger
var cacheMedia = false

func checkLogin(r *http.Request) (string, error) {
    cookie, err := r.Cookie("username")
    if err != nil {
        return "", err
    } else {
        return cookie.Value, nil
    }
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func errResponse(err error) response {
    var res response
    res.Status = "error"
    res.Error = err.Error()
    return res
}

func AddMediaHandler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    username, err := checkLogin(r)
    if err != nil {
        Log.Error(err)
        encodeResponse(w, errResponse(err))
        return
    }
    if r.MultipartForm == nil {
        err = r.ParseMultipartForm(defaultMaxMemory)
        if err != nil {
            encodeResponse(w, errResponse(err))
            return
        }
    }
    if r.MultipartForm.File == nil || r.MultipartForm.File[fileKey] == nil {
        err = errors.New("No file found")
        encodeResponse(w, errResponse(err))
        return
    }
    // File exists.
    oid := objectid.New()
    elapsed := time.Since(start)
    Log.Info("Add Media (pre-insert) elapsed: " + elapsed.String())
    var res response
    res.Status = "OK"
    res.ID = oid.Hex()
    encodeResponse(w, res)
    go insertWithTimer(r, oid, username, start)
}

func insertWithTimer(r *http.Request, oid objectid.ObjectID,
    username string, start time.Time) {
    content, header, err := r.FormFile(fileKey) // Get binary payload.
    if err != nil {
        Log.Error(err)
        return
    }
    var m media.Media
    m.ID = oid
    defer content.Close()
    Log.Debug(header.Header)
    bufContent := bytes.NewBuffer(nil)
    if _, err := io.Copy(bufContent, content); err != nil {
        Log.Error(err)
        return
    }
    buf := bufContent.Bytes()
    if header != nil {
        m.ContentType = header.Header["Content-Type"][0]
    }
    m.Content = buf
    m.Username = username
    // Add the Media.
    err = insertMedia(m)
    if err != nil {
       Log.Error(err.Error())
    }
    elapsed := time.Since(start)
    Log.Info("Add Media (post-insert) elapsed: " + elapsed.String())
}

func insertMedia(m media.Media) (error) {
    start := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        return err
    }
    db := client.Database("twitter")
    col := db.Collection("media")
    _, err = col.InsertOne(context.Background(), &m)
    elapsed := time.Since(start)
    Log.Info("Insert media time elapsed: " + elapsed.String())
    if err == nil && cacheMedia { // Cache
        err = memcached.Set(media.CacheKey(m.ID.Hex()), &m)
        if err != nil {
            Log.Error(err)
        }
    }
    return err
}
