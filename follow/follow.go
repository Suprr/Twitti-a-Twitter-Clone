package follow

import (
    "context"
    "net/http"
    "time"
    "errors"
    "github.com/sirupsen/logrus"
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "github.com/mongodb/mongo-go-driver/mongo"
    "TwitterClone/wrappers"
    //"TwitterClone/user"
    "TwitterClone/user/user_endpoints/followInfo"
    "TwitterClone/memcached"
)

type Request struct {
    Username *string `json:"username"`
    Follow *bool `json:"follow"`
}

type response struct {
    Status string `json:"status"`
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

func getUserOID(client *mongo.Client, username string) (objectid.ObjectID, error) {
    var nilOID objectid.ObjectID
    result := bson.NewDocument()
    db := client.Database("twitter")
    filter := bson.NewDocument(
        bson.EC.String("username", username))
    err := db.Collection("usernames").FindOne(context.Background(), filter).Decode(result)
    if err != nil {
        return nilOID, err
    }
    elem, err := result.Lookup("_id")
    if err != nil {
        return nilOID, err
    }
    oid := elem.Value().ObjectID()
    return oid, nil
}

func followUser(currentUser string, userToFol string, follow bool) error {
  //dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        return nil
    }
    db := client.Database("twitter")
    currUserOID, err := getUserOID(client, currentUser)
    if err != nil {
        return err
    }
    userToFolOID, err := getUserOID(client, userToFol)
    if err != nil {
        return err
    }
    var countInc int32
    if follow {
        countInc = 1
    } else {
        countInc = -1
    }
    err = updateFollow(client, currentUser, userToFol, follow)
    if err != nil {
        return err
    }
    filter := bson.NewDocument(
        bson.EC.ObjectID("_id", currUserOID))
    update := bson.NewDocument(
        bson.EC.SubDocumentFromElements("$inc",
            bson.EC.Int32("followingCount", countInc)))
    coll := db.Collection("users")
    err = UpdateOne(coll, filter, update)
    if err != nil {
        return err
    }
    // Update follower count.
    filter = bson.NewDocument(
        bson.EC.ObjectID("_id", userToFolOID))
    update = bson.NewDocument(
        bson.EC.SubDocumentFromElements("$inc",
            bson.EC.Int32("followerCount", countInc)))
    go UpdateOne(coll, filter, update)
    return nil
}

func insertFollowDocs(client *mongo.Client, currentUser string, userToFol string, follow bool) error {
    db := client.Database("twitter")
    followersCol := db.Collection("followers")
    followingCol := db.Collection("following")
    dbStart := time.Now()
    followerDoc := bson.NewDocument(
        bson.EC.String("user", userToFol),
        bson.EC.String("follower", currentUser))
    followingDoc := bson.NewDocument(
        bson.EC.String("user", currentUser),
        bson.EC.String("following", userToFol))
    if follow {
        _, err := followersCol.InsertOne(context.Background(), followerDoc)
        if err != nil {
           return err
        }
        _, err = followingCol.InsertOne(context.Background(), followingDoc)
        if err != nil {
             return err
        }
        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"endpoint":"item",
            "timeElapsed":elapsed.String()}).Info(
                "Inserting a follow time elapsed")
    } else {
        // Delete follow doc.
          _, err := followersCol.DeleteOne(context.Background(), followerDoc)
          if err != nil {
             return err
          }
          _, err = followingCol.DeleteOne(context.Background(), followingDoc)
          if err != nil {
               return err
          }
        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"endpoint":"item",
            "timeElapsed":elapsed.String()}).Info("Delete follow time elapsed")
    }
    memcached.Delete(followInfo.FollowingCacheKey(currentUser))
    memcached.Delete(followInfo.FollowersCacheKey(currentUser))
    return nil
}

func updateFollow(client *mongo.Client, currentUser string, userToFol string, follow bool) error {
    db := client.Database("twitter")
    followersCol := db.Collection("followers")
    followingCol := db.Collection("following")
    //dbStart := time.Now()
    followerDoc := bson.NewDocument(
        bson.EC.String("user", userToFol),
        bson.EC.String("follower", currentUser))
    followingDoc := bson.NewDocument(
        bson.EC.String("user", currentUser),
        bson.EC.String("following", userToFol))
    resDoc := bson.NewDocument()
    err := followersCol.FindOne(context.Background(), followerDoc).Decode(resDoc)
    Log.Debug(resDoc)
    if (err == nil) == follow {
        return errors.New("Tried to duplicate follow or unfollow")
    }
    resDoc.Reset()
    err = followingCol.FindOne(context.Background(), followingDoc).Decode(resDoc)
    if (err == nil) == follow {
        return errors.New("Tried to duplicate follow or unfollow")
    }
    go insertFollowDocs(client, currentUser, userToFol, follow)
    return nil
}

func UpdateOne(coll *mongo.Collection, filter interface{}, update interface{}) error {
  dbStart := time.Now()
    result, err := coll.UpdateMany( // UpdateMany is temporary.
        context.Background(),
        filter, update)

      elapsed := time.Since(dbStart)
      Log.WithFields(logrus.Fields{"endpoint": "follow", "timeElapsed":elapsed.String()}).Info("updating time elapsed")
    var success = false
    if result != nil {
        Log.Debug(*result)
        success = result.ModifiedCount == 1
    }
    if err != nil {
        return err
    } else if !success {
        return errors.New("Database is operating normally, but follow update " +
        "operation failed.")
    } else {
        return nil;
    }
}

func decodeRequest(r *http.Request) (Request, error) {
    decoder := json.NewDecoder(r.Body)
    var it Request
    err := decoder.Decode(&it)
    if it.Follow == nil{
      def := new(bool)
      *def = true
      it.Follow = def
    }
    return it, err
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func FollowHandler(w http.ResponseWriter, r *http.Request) {
    Log.SetLevel(logrus.DebugLevel)
    start := time.Now()
    var res response
    username, err := checkLogin(r)
    if err != nil {
        res.Status = "error"
        res.Error = "User not logged in."
    } else {
        it, err := decodeRequest(r)
        if (err != nil) {
            res.Status = "error"
            res.Error = "JSON decoding error."
        } else {
            Log.WithFields(logrus.Fields{
                "username": *it.Username,
                "follow": *it.Follow,
                "currentUser": username}).Info()
            res = followEndpoint(username, it)
        }
    }

    elapsed := time.Since(start)
    Log.Info("AddItem elapsed: " + elapsed.String())
    encodeResponse(w, res)
}

func followEndpoint(username string,it Request) response {
    var res response
    valid := validateReq(it)
    if valid {
        // Add the Item.
        err := followUser(username, *it.Username, *it.Follow)
        if err != nil {
            res.Status = "error"
            res.Error = err.Error()
        } else {
            res.Status = "OK"
        }
    } else {
        res.Status = "error"
        res.Error = "Invalid request."
        Log.Info("Invalid request!")
    }
    return res
}

func validateReq(it Request) bool {
    return it.Username != nil && it.Follow != nil
}
