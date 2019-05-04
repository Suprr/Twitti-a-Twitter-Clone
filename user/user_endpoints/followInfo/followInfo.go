package followInfo

import (
    "context"
    "net/http"
    "errors"
    logrus "github.com/sirupsen/logrus"
    "encoding/json"
    "TwitterClone/wrappers"
    "time"
    "github.com/gorilla/mux"
    "github.com/mongodb/mongo-go-driver/bson"
    "github.com/mongodb/mongo-go-driver/mongo"
    "strconv"
    "TwitterClone/memcached"
)

type followList struct {
    Following []string `bson:"following,omitempty"`
    Followers []string `bson:"followers,omitempty"`
}

type response struct {
    Status string `json:"status"`
    Users []string `json:"users"`
    Error string `json:"error,omitempty"`
}
var Log *logrus.Logger
func main() {
    Log.SetLevel(logrus.ErrorLevel)
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func getUsername(r *http.Request) (string){
    vars := mux.Vars(r)
    username := vars["username"]
    Log.Debug(username)
    return username
}

func checkLimit(r *http.Request) (int64,error){
    limit, err := strconv.ParseInt(r.URL.Query().Get("limit"),10,64)
    if err != nil{
        limit = 50
    }else if limit != 0 && limit > 200{
        Log.Error("Limit exceeds 200")
        return 0,errors.New("Limit exceeds 200")
    }
    Log.Info(limit)
    return limit,nil
}

func GetFollowingHandler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    var resp response
    lim, e := checkLimit(r)
    if e != nil{
        Log.Info(e)
        resp.Status = "error"
        resp.Error = e.Error()
        encodeResponse(w,resp)
    }
    username := getUsername(r)
    var l followList
    memstart := time.Now()
    err := memcached.Get(FollowingCacheKey(username), &l)
    elapsed := time.Since(memstart)
    Log.WithFields(logrus.Fields{"endpoint":"item",
    "timeElapsed":elapsed.String()}).Info("Get item from memcached")
    if err != nil {
        Log.Debug(err)
        list, err := findUserFollowing(username,lim)
        if err != nil {
            Log.Info(err)
            resp.Status = "error"
            resp.Error = err.Error()
            encodeResponse(w,resp)
            elapsed := time.Since(start)
            Log.Info("Get Following elapsed: " + elapsed.String())
            return
        } else{
            resp.Status = "OK"
            resp.Users = list
        }
    } else {
        Log.Debug("Cache hit")
        resp.Status = "OK"
        resp.Users = l.Following
    }
    encodeResponse(w, resp)
    elapsed = time.Since(start)
    Log.Info("Get Following elapsed: " + elapsed.String())
}


func GetFollowersHandler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    var resp response
    lim, e := checkLimit(r)
    if e != nil{
        Log.Info(e)
        resp.Status = "error"
        resp.Error = e.Error()
        encodeResponse(w,resp)
    }
    username := getUsername(r)
    var l followList
    memstart := time.Now()
    err := memcached.Get(FollowersCacheKey(username), &l)
    elapsed := time.Since(memstart)
    Log.WithFields(logrus.Fields{"endpoint":"item",
    "timeElapsed":elapsed.String()}).Info("Get item from memcached")
    if err != nil {
        Log.Debug(err)
        list, err := findUserFollowers(username,lim)
        if err != nil {
            Log.Info(err)
            resp.Status = "error"
            resp.Error = err.Error()

            elapsed := time.Since(start)
            Log.Info("Get Followers elapsed: " + elapsed.String())
            encodeResponse(w,resp)
            return
        }else{
            resp.Status = "OK"
            resp.Users = list
        }
    } else {
        Log.Debug("Cache hit")
        resp.Status = "OK"
        resp.Users = l.Followers
    }

    elapsed = time.Since(start)
    Log.Info("Get Followers elapsed: " + elapsed.String())
    encodeResponse(w, resp)
}


func findUserFollowing(username string, lim int64) ([]string, error) {
    following,err := findUserFollow(username,"following", lim)
    list := []string{}
    if err != nil{
        Log.Error(err)
        return nil, err
    }
    if following != nil{
        return following, nil
    }else{
        return list, nil
    }
}

func findUserFollowers(username string, lim int64) ([]string, error) {
    followers,err := findUserFollow(username, "followers", lim)
    list := []string{}
    if err != nil{
        Log.Error(err)
        return nil, err
    }

    if followers != nil{
        return followers,nil
    }else{
        return list, nil
    }
}

func findUserFollow(username string, follow string, lim int64) ([]string,error){
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        return nil,err
    }
    db := client.Database("twitter")
    coll := db.Collection(follow)
    filter := bson.NewDocument(bson.EC.String("user", username))

    proj := bson.NewDocument(bson.EC.Int32("_id",0), bson.EC.Int32("user", 0))
    option, err := mongo.Opt.Projection(proj)
    if err != nil {
        return nil,err
    }
    cursor, err := coll.Find(context.Background(), filter, option)
    if err != nil{
        elapsed := time.Since(dbStart)
        Log.WithFields(logrus.Fields{"msg":"Get Follower/ing time elapsed", "timeElapsed":elapsed.String()}).Error("Could not find follower/ing list")
        Log.Error(err)
        return nil, err
    }
    var sTerm string
    if follow == "following" {
        sTerm = follow
    } else {
        sTerm = "follower"
    }
    var fArr []string
    doc := bson.NewDocument()
    for cursor.Next(context.Background()) {
        doc.Reset()
        err := cursor.Decode(doc)
        if err != nil {
            Log.Error(err)
            return nil, err
        }
        f, err := doc.Lookup(sTerm)
        if err != nil {
            Log.Error(err)
            return nil, err
        }
        fArr = append(fArr, f.Value().StringValue())
    }
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"msg":"Get Followier/ing time elapsed", "timeElapsed":elapsed.String()}).Info()
    if(follow == "followers"){
        // Set in cache
        err = memcached.Set(FollowersCacheKey(username), &fArr)
    } else {
        // Set in cache
        err = memcached.Set(FollowingCacheKey(username), &fArr)
    }
    if err != nil {
        Log.Error(err)
    }
    return fArr, nil
}

func FollowingCacheKey(username string) string {
    return username + "_following"
}

func FollowersCacheKey(username string) string {
    return username + "_followers"
}
