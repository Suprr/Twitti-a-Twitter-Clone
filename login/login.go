package login

import (
    "context"
    "bytes"
    "time"
    "github.com/sirupsen/logrus"
    "net/http"
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson"
    "crypto/md5"
    "TwitterClone/user"
    "TwitterClone/wrappers"
)


type response struct {
    Status string `json:"status"`
    Error string `json:"error,omitempty"`
}

type userDetails struct {
    Username *string `json:"username"`
    Password *string `json:"password"`
}
var Log *logrus.Logger

func authUser(details userDetails) bool {
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error("Mongodb error")
        return false
    }
    dbStart := time.Now()
    db := client.Database("twitter")
    coll := db.Collection("usernames")
    filter := bson.NewDocument(bson.EC.String("username", *details.Username))
    result := bson.NewDocument()
    err = coll.FindOne(context.Background(), filter).Decode(result)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint" : "login","msg":"Check if user exists in DB time elapsed",
        "timeElapsed":elapsed.String()}).Info()
    if err != nil {
        return false
    }
    elem, err := result.Lookup("_id")
    if err != nil {
        Log.Error(err)
        return false
    }
    oid := elem.Value().ObjectID()
    filter = bson.NewDocument(bson.EC.ObjectID("_id", oid))
    var user user.User
    coll = db.Collection("users")
    err = coll.FindOne(context.Background(), filter).Decode(&user)
    if err != nil {
        Log.Error(err)
        return false
    } else {
        Log.Debug(user)
    }
    encStart := time.Now()
    inputPw := md5.Sum([]byte(*details.Password))
    authed := user.Verified && bytes.Equal(inputPw[:], user.Password)
    elapsed = time.Since(encStart)
    Log.Info("encryption elapsed: " + elapsed.String())
    return authed
}

func decodeRequest(r *http.Request) (userDetails, error) {
    decoder := json.NewDecoder(r.Body)
    var details userDetails
    err := decoder.Decode(&details)
    return details, err
}
func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
    Log.SetLevel(logrus.InfoLevel)
    timeStart := time.Now()
    var res response
    details, err := decodeRequest(r)
    if (err != nil) {
        res.Status = "error"
        res.Error = "Error decoding json."
    } else if validateDetails(details) {
        res = loginEndpoint(details)
    } else {
        res.Status = "error"
        res.Error = "Invalid request."
    }
    if (res.Status == "OK") {
        var cookie http.Cookie
        cookie.Name = "username"
        cookie.Value = *details.Username
        http.SetCookie(w, &cookie)
    }
    elapsed := time.Since(timeStart)
    Log.Info("elapsed: " + elapsed.String())
    encodeResponse(w, res)
}

func loginEndpoint(details userDetails) (response) {
    var res response
    // Check username and password against database.
    shouldLogin := authUser(details)
    if (shouldLogin) {
        res.Status = "OK"
    } else {
        res.Status = "error"
        res.Error = "User not found or incorrect password."
    }
    return res
}

func validateDetails(details userDetails) bool {
    return (details.Username != nil && details.Password != nil)
}
