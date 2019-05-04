package item_endpoints

import (
    "context"
    "errors"
    logrus "github.com/sirupsen/logrus"
    "net/http"
    "time"
    "encoding/json"
    "github.com/gorilla/mux"
    "github.com/mongodb/mongo-go-driver/bson"
    "github.com/mongodb/mongo-go-driver/mongo"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "TwitterClone/wrappers"
    "TwitterClone/item"
    "TwitterClone/memcached"
    //"github.com/olivere/elastic"
)

type like struct {
    ItemID objectid.ObjectID `bson:"item_id"`
    Username string `bson:"username"`
}

type response struct {
    Status string `json:"status"`
    Item item.Item `json:"item,omitempty"`
    Error string `json:"error,omitempty"`
}

//response for Like
type responseL struct {
    Status string `json:"status"`
    Error string `json:"error,omitempty"`
}

//post params for like
type Req struct {
    Like *bool `json:"like"`
}
var Log *logrus.Logger


//LIKE ITEM FUNCTIONS START HERE

func LikeItemHandler(w http.ResponseWriter, r *http.Request) {
    Log.SetLevel(logrus.InfoLevel)
    start := time.Now()
    id := mux.Vars(r)["id"]
    Log.Debug(id)

    var res responseL
    username, err := checkLogin(r)
    if err != nil {
        Log.Error("User not logged in")
        res.Status = "error"
        res.Error = "User not logged in."
    } else {
        req, err := decodeRequest(r)
        if err != nil {
            Log.Error(r)
            Log.Error("JSON decoding error")
            res.Status = "error"
            res.Error = "JSON decoding error."
        } else {
            res = likeItem(id, username, *req.Like)
        }
    }

    elapsed := time.Since(start)
    Log.Info("AddItem elapsed: " + elapsed.String())
    encodeResponse(w, res)
}

func decodeRequest(r *http.Request) (Req, error) {
    decoder := json.NewDecoder(r.Body)
    var like Req
    err := decoder.Decode(&like)
    return like, err
}

func likeItem(itemID string, username string, shouldLike bool) responseL {
    var resp responseL
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
       Log.Error(err)
       resp.Status = "error"
       resp.Error = err.Error()
       return resp
    }
    db := client.Database("twitter")
    itemOID, err := objectid.FromHex(itemID)
    if err != nil {
       Log.Error(err)
       resp.Status = "error"
       resp.Error = err.Error()
       return resp
    }
    dbStart = time.Now()
    col := db.Collection("likes")
    var like like
    like.ItemID = itemOID
    like.Username = username
    filter := bson.NewDocument(
        bson.EC.ObjectID("item_id", itemOID),
        bson.EC.String("username", username))
    resDoc := bson.NewDocument()
    err = col.FindOne(context.Background(), filter).Decode(resDoc)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint":"item",
        "timeElapsed":elapsed.String()}).Info(
            "Getting like from likes coll.")
    if (err == nil) == shouldLike {
        // Like already exists
        err = errors.New("Tried to duplicate like or dislike " +
        " nonexistent like for  username: " + like.Username +
        " and itemID: " + like.ItemID.Hex())
        Log.Error(err)
        resp.Status = "error"
        resp.Error = err.Error()
        return resp
    } else {
        Log.Error(err)
        resp.Status = "OK"
        go updateLike(client, like, shouldLike)
        return resp
    }
}

func updateLike(client *mongo.Client,
    like like, shouldLike bool) {
    db := client.Database("twitter")
    coll := db.Collection("likes")
    dbStart := time.Now()
    if shouldLike {
        _, err := coll.InsertOne(context.Background(), &like)
        if err != nil {
           Log.Error(err)
        }
        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"endpoint":"item",
            "timeElapsed":elapsed.String()}).Info(
                "Inserting a like time elapsed")
    } else {
        // Delete like object (itemid, username).
        _, err := coll.DeleteOne(context.Background(), &like)
        if err != nil {
            Log.Error(err)
        }
        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"endpoint":"item",
            "timeElapsed":elapsed.String()}).Info("Delete like time elapsed")
    }
    // Update the tweet.
    coll = db.Collection("tweets")
    // Are we incrementing or decrementing the number of likes?
    var countInc int32
    if shouldLike {
        countInc = 1
    } else {
        countInc = -1
    }
    filter := bson.NewDocument(bson.EC.ObjectID("_id", like.ItemID))
    update := bson.NewDocument(bson.EC.SubDocumentFromElements("$inc",
        bson.EC.Int32("property.likes", countInc)))
    err := UpdateOne(coll, filter, update)
    if err != nil {
        Log.Error(err)
    } else {
        err := memcached.Delete(item.CacheKey(like.ItemID.Hex()))
        if err != nil {
            Log.Error(err)
        }
    }
}

func UpdateOne(coll *mongo.Collection, filter interface{}, update interface{}) error {
  dbStart := time.Now()
    result, err := coll.UpdateOne(
        context.Background(),
        filter, update)
        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"endpoint":"item", "timeElapsed":elapsed.String()}).Info("Update time elapsed")
    var success = false
    if result != nil {
        Log.Debug(*result)
        success = result.ModifiedCount == 1
    }
    if err != nil {
        return err
    } else if !success {
        return errors.New("Database is operating normally, but like update " +
        "operation failed.")
    } else {
        return nil;
    }
}

//LIKE ITEM ENDPOINT ENDS HERE

//GET ITEM FUNCTIONS START HERE
func GetItemHandler(w http.ResponseWriter, r *http.Request) {
    Log.SetLevel(logrus.InfoLevel)
    var res response
    id := mux.Vars(r)["id"]
    Log.Debug(id)
    var it item.Item
    start := time.Now()
    err := memcached.Get(item.CacheKey(id), &it)
    elapsed := time.Since(start)
    Log.WithFields(logrus.Fields{"endpoint":"item",
        "timeElapsed":elapsed.String()}).Info("Get item from memcached")
    if err != nil {
        Log.Debug(err)
        res = getItemFromMongo(id)
    } else {
        Log.Debug("Cache hit")
        res.Status = "OK"
        res.Item = it
    }
    encodeResponse(w, res)
}

func getItemFromMongo(id string) response {
    var it item.Item
    var resp response
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error(err)
        resp.Status = "error"
        resp.Error = err.Error()
        return resp
    }
    db := client.Database("twitter")
    col := db.Collection("tweets")
    objectid, err := objectid.FromHex(id)
    Log.Debug(objectid)

    if err != nil {
        Log.Error(err)
        resp.Status = "error"
        resp.Error = err.Error()
        return resp
    }
    filter := bson.NewDocument(bson.EC.ObjectID("_id", objectid))
    err = col.FindOne(
        context.Background(),
        filter).Decode(&it)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint":"item",
        "timeElapsed":elapsed.String()}).Info("Get item from Mongo")
    if err != nil {
        Log.Error(err)
        resp.Status = "error"
        resp.Error = err.Error()
        return resp
    }
    resp.Status = "OK"
    resp.Item = it
    // Set in cache
    err = memcached.Set(item.CacheKey(id), &it)
    if err != nil {
        Log.Error(err)
    }
    return resp
}
//GET ITEM FUNCTIONS END HERE

//DELETE ITEM FUNCTIONS START HERE
func DeleteItemHandler(w http.ResponseWriter, r *http.Request) {
    username, err := checkLogin(r)
    if err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }
    var statusCode int
    id := mux.Vars(r)["id"]
    Log.Debug(id)
    statusCode = deleteItem(username, id)
    w.WriteHeader(statusCode)
}


func deleteItem(username string, id string) int {
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error(err)
        return http.StatusInternalServerError
    }
    db := client.Database("twitter")
    col := db.Collection("tweets")
    objectid, err := objectid.FromHex(id)
    if err != nil {
        Log.Error(err)
        return http.StatusBadRequest
    }
    // Pull item from database.
    var it item.Item
    doc := bson.NewDocument(
        bson.EC.ObjectID("_id", objectid),
        bson.EC.String("username", username))
    err = col.FindOne(context.Background(), doc).Decode(&it)
    if err != nil {
        Log.Info("item does not exist")
        Log.Error(err)
        return http.StatusBadRequest
    }
    // Delete associated media, if it exists.
    var result *mongo.UpdateResult
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
            bson.EC.SubDocumentFromElements("$pull",
            bson.EC.ObjectID("item_ids", it.ID)))
            result, err = col.UpdateMany(context.Background(), filter, update)
            Log.Debug(result)
    }
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint":"item", "timeElapsed":elapsed.String()}).Info("Delete associated media elapsed")
    if err != nil {
        Log.Error(err)
        return http.StatusInternalServerError
    }
    // Successfully deleted media ids.
    // Delete actual item.
    doc = bson.NewDocument(bson.EC.ObjectID("_id", objectid))
    _, err = col.DeleteOne(
        context.Background(),
        doc)

      elapsed = time.Since(dbStart)
      Log.WithFields(logrus.Fields{"endpoint":"item", "timeElapsed":elapsed.String()}).Info("Delete actual item time elapsed")
    if err != nil {
        Log.Error("Did not find item when deleting.")
        Log.Error(err)
        return http.StatusInternalServerError
    }
    return http.StatusOK
}
//DELETE ITEM FUNCTIONS END HERE

//MISC
func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func validateItem(it item.Item) bool {
    valid := true
    return valid
}

func checkLogin(r *http.Request) (string, error) {
    cookie, err := r.Cookie("username")
    if err != nil {
        return "", err
    } else {
        return cookie.Value, nil
    }
}
