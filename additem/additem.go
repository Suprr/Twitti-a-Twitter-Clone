package additem

import (
    "context"
    "net/http"
    "time"
    "errors"
    "github.com/sirupsen/logrus"
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "github.com/mongodb/mongo-go-driver/bson"
    "github.com/mongodb/mongo-go-driver/mongo"
    "TwitterClone/wrappers"
    "TwitterClone/item"
    "TwitterClone/memcached"
)

type request struct {
    Content *string `json:"content"`
    ChildType *string `json:"childType,omitempty"`
    ParentID *string `json:"parent,omitempty"`
    MediaIDs *[]string `json:"media,omitempty"`
}

type response struct {
    Status string `json:"status"`
    ID string  `json:"id,omitempty"`
    Error string `json:"error,omitempty"`
}

var Log *logrus.Logger

func checkLogin(r *http.Request) (string, error) {
    cookie, err := r.Cookie("username")
    if err != nil {
        return "", err
    } else {
        return cookie.Value, nil
    }
}


func insertItem(it item.Item) (item.Item, error) {
    var nilItem item.Item
    start := time.Now()
    client, err := wrappers.NewClient()
    db := client.Database("twitter")
    col := db.Collection("tweets")
    it.Timestamp = time.Now().Unix()
    Log.Debug(it)
    dbStart := time.Now()
    _, err = col.InsertOne(context.Background(), &it)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint": "additem",
        "timeElapsed":elapsed.String()}).Debug("insert item time elapsed")

    if err != nil {
        Log.Error(err)
        return nilItem, err
    }
    var result *mongo.UpdateResult
    if it.ChildType == "retweet" {
        // Increment retweet counter of parent.
        filter := bson.NewDocument(bson.EC.ObjectID("_id", it.ParentID))
        update := bson.NewDocument(
            bson.EC.SubDocumentFromElements("$inc",
            bson.EC.Int32("retweeted", 1)))
            dbStart := time.Now()
        result, err = col.UpdateOne(context.Background(), filter, update)

        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"endpoint": "addItem",
            "timeElapsed":elapsed.String()}).Info("retweet increment time elapsed")
        if err != nil {
            Log.Error(err)
            return nilItem, err
        } else if result.ModifiedCount != 1 {
            err = errors.New("Referenced Parent ID not found")
            return nilItem, err
        } else {
            if err != nil {
                Log.Error(err)
                return nilItem, err
            } else {
                Log.Debug(update)
            }
        }
    }
    // Update media which item references.
    if it.MediaIDs != nil {
        col = db.Collection("media")
        bArray := bson.NewArray()
        for _, mOID := range it.MediaIDs {
            bArray.Append(bson.VC.ObjectID(mOID))
        }
        filter := bson.NewDocument(
            bson.EC.SubDocumentFromElements("_id",
            bson.EC.Array("$in", bArray)))
        update := bson.NewDocument(
            bson.EC.SubDocumentFromElements("$addToSet",
            bson.EC.ObjectID("item_ids", it.ID)))
            dbStart :=time.Now()
            result, err = col.UpdateMany(context.Background(), filter, update)

            elapsed := time.Since(dbStart)
            Log.WithFields(logrus.Fields{"endpoint": "addItem",
                "timeElapsed":elapsed.String()}).Info("update media time elapsed")
        if err != nil {
            return nilItem, err
        } else if result.ModifiedCount != 1 {
            err = errors.New("Media item_ids not updated. Probably invalid ids.")
            Log.Error(err)
            return nilItem, err
        }
    }
    elapsed = time.Since(start)
    Log.Info("AddItem elapsed: " + elapsed.String())
    return it, nil
}

func decodeRequest(r *http.Request) (request, error) {
    decoder := json.NewDecoder(r.Body)
    var req request
    err := decoder.Decode(&req)
    return req, err
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func AddItemHandler(w http.ResponseWriter, r *http.Request) {
    Log.SetLevel(logrus.InfoLevel)
    var res response
    username, err := checkLogin(r)
    start := time.Now()
    if err != nil {
        Log.Error("User not logged in")
        res.Status = "error"
        res.Error = "User not logged in."
        encodeResponse(w, res)
        return
    }
    req, err := decodeRequest(r)
    if (err != nil) {
        Log.Error("JSON decoding error")
        res.Status = "error"
        res.Error = "JSON decoding error."
        encodeResponse(w, res)
        return
    }
    pOID, mOIDs, err := validateRequest(req)
    if err == nil {
        oid := objectid.New()
        res.Status = "OK"
        res.ID = oid.Hex()
        elapsed := time.Since(start)
        Log.WithFields(logrus.Fields{"endpoint": "additem",
            "timeElapsed":elapsed.String()}).Info("pre-insert")
        encodeResponse(w, res) // Cheat
        go insertWithTimer(req, oid, pOID, mOIDs, username, start)
    } else {
        res.Status = "error"
        res.Error = err.Error()
        encodeResponse(w, res)
    }
}

func insertWithTimer(req request, oid objectid.ObjectID,
    pOID objectid.ObjectID, mOIDs []objectid.ObjectID,
    username string, start time.Time) {
    var it item.Item
    it.Content = *req.Content
    if req.ChildType != nil {
        it.ChildType = *req.ChildType
    }
    if req.ParentID != nil {
        it.ParentID = pOID
        Log.Debug(*req.ParentID)
    }
    if req.MediaIDs != nil {
        it.MediaIDs = mOIDs
    }
    it.Username = username
    logrus.Debug(it)
    // Add the Item.
    Log.Debug(oid)
    it.ID = oid
    it, err := insertItem(it)
    if err != nil {
        Log.Error(err)
    } else {
        // Cache inserted item.
        Log.Debug("Caching item")
        err = memcached.Set(item.CacheKey(it.ID.Hex()), &it)
        if err != nil {
            Log.Error(err)
        }
    }
    elapsed := time.Since(start)
    Log.WithFields(logrus.Fields{"endpoint": "additem",
        "timeElapsed":elapsed.String()}).Info("post-insert")
}


func validateRequest(req request) (objectid.ObjectID, []objectid.ObjectID, error) {
    var pOID objectid.ObjectID
    var mOIDs []objectid.ObjectID
    var err error
    if req.Content == nil {
        err = errors.New("Null content")
    } else if req.ChildType == nil {
        if req.ParentID != nil {
            err = errors.New("Parent not null when childType is")
        }
    } else if (*req.ChildType != "retweet" && *req.ChildType != "reply") {
        err = errors.New("Child type not valid")
    } else if req.ParentID == nil {
        err = errors.New("Parent must be set when child type exists.")
    } else {
        pOID, err = objectid.FromHex(*req.ParentID)
    }
    if err == nil && req.MediaIDs != nil {
        for _, mID := range *req.MediaIDs {
            mOID, idErr := objectid.FromHex(mID)
            if idErr == nil {
                mOIDs = append(mOIDs, mOID)
            } else {
                err = idErr
                break
            }
        }
    }
    return pOID, mOIDs, err
}
