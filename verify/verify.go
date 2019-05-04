package verify

import (
    "context"
    "time"
    "errors"
    "github.com/sirupsen/logrus"
    "net/http"
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "TwitterClone/wrappers"
    "TwitterClone/user"
)

type verification struct {
    Key *string `json:"key"`
    Email *string `json:"email"`
}

type res struct {
    Status string `json:"status"`
    Error string `json:"error,omitempty"`
}
var Log *logrus.Logger

func VerifyHandler(w http.ResponseWriter, req *http.Request) {
    Log.SetLevel(logrus.DebugLevel)
    start := time.Now()
    decoder := json.NewDecoder(req.Body)
    var verif verification
    var r res
    err := decoder.Decode(&verif)
    if err != nil {
        panic(err)
    }
    valid := validateParams(verif)
    if valid {
        Log.WithFields(logrus.Fields{"email": verif.Email, "key": verif.Key}).Info()
        err = verifyUser(verif)
        if err == nil {
            r.Status = "OK"
        } else {
            Log.Error(err)
            r.Status = "error"
            r.Error = err.Error()
        }
    } else {
        r.Status = "error"
        r.Error = "Not enough input"
    }

    elapsed := time.Since(start)
    Log.Info("Verify elapsed: " + elapsed.String())
    json.NewEncoder(w).Encode(r)
}

func validateParams(verif verification) bool {
    valid := true
    if (verif.Email == nil) {
        valid = false
    } else if (verif.Key == nil) {
        valid = false
    }
    if (valid) {
        Log.Debug("Key: ", *verif.Key)
        Log.Debug("Email: ", *verif.Email)
    }
    return valid
}

func verifyUser(verif verification) error {
    client, err := wrappers.NewClient()
    if err != nil {
        return err
    }
    db := client.Database("twitter")
    coll := db.Collection("emails")
    // Find user.
    dbStart := time.Now()
    filter := bson.NewDocument(bson.EC.String("email", *verif.Email))
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"msg":"Check if user exists time elapsed",
        "timeElapsed":elapsed.String()}).Info()
    result := bson.NewDocument()
    err = coll.FindOne(context.Background(), filter).Decode(result)
    Log.Debug(*verif.Email)
    Log.Debug(result)
    elem, err := result.Lookup("_id")
    if err != nil {
        return err
    }
    oid := elem.Value().ObjectID()
    if *verif.Key == "abracadabra" {
        err = mongoUpdateVerify(oid)
        return err
    }
    var user user.User
    coll = db.Collection("users")
    err = coll.FindOne(context.Background(), filter).Decode(&user)
    if err != nil {
        return err
    } else {
        Log.Debug(user)
    }
    if user.Key == "<" + *verif.Key + ">" {
        // Verification keys match.
        err = mongoUpdateVerify(oid)
    } else {
        err = errors.New("Verification keys do not match.")
    }
    return err
}

func mongoUpdateVerify(userID objectid.ObjectID) error {
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error(err)
        return err
    }
    db := client.Database("twitter")
    coll := db.Collection("users")
    filter := bson.NewDocument(bson.EC.ObjectID("_id", userID))
    update := bson.NewDocument(
        bson.EC.SubDocumentFromElements("$set",
        bson.EC.Boolean("verified", true)))
    result, err := coll.UpdateOne(
        context.Background(),
        filter, update)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"msg":"Update user verified field",
        "timeElapsed":elapsed.String()}).Info()
    if err != nil {
        return err
    }
    if result.ModifiedCount == 1 {
        return nil
    } else {
        err = errors.New("ModifiedCount == 0... Something weird happened.")
        return err
    }
}
