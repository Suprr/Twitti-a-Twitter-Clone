package search

import (
        "time"
        "context"
        "net/http"
        "reflect"
        "github.com/sirupsen/logrus"
        "encoding/json"
        "github.com/mongodb/mongo-go-driver/mongo"
        "github.com/mongodb/mongo-go-driver/bson"
        "github.com/mongodb/mongo-go-driver/bson/objectid"
        "TwitterClone/wrappers"
        "TwitterClone/item"
        "github.com/olivere/elastic"
)

type params struct {
        Timestamp int64 `json:"timestamp,string"`
        Limit int64 `json:"limit,omitempty"`
        Q string `json:"q,omitempty"`
        Un string `json:"username,omitempty"`
        Following *bool `json:"following,omitempty"`
        Rank *string `json:"rank,omitempty"`
        Replies *bool `json:"replies"`
        HasMedia bool `json:"hasMedia"`
        Parent *string `json:"parent"`
}

type property struct {
    Likes int `json:"likes"`
}

type followingList struct {
    Following []string `bson:"following,omitempty"`
}

type res struct {
    Status string `json:"status"`
    Items []item.Item `json:"items"`
    Error string `json:"error,omitempty"`
}

var Log = logrus.New()

func getUsername(r *http.Request) (string, error) {
        cookie, err := r.Cookie("username")
        if err != nil {
                return "", err
        } else {
                return cookie.Value, nil
        }
}

func SearchHandler(w http.ResponseWriter, req *http.Request) {
    startTime := time.Now()
    decoder := json.NewDecoder(req.Body)
    var start params
    var r res
    err := decoder.Decode(&start)
    if err != nil {
         r.Status = "error"
         r.Error = err.Error()
         Log.Error("Could not decode JSON")
         json.NewEncoder(w).Encode(r)
         return
    }
    //Error checking and defaulting the parameters
    if(start.Timestamp == 0){
        start.Timestamp = time.Now().Unix()
    }
    if(start.Limit == 0){
        start.Limit = 25
    }
    if(start.Limit > 100){
        r.Status = "error"
        r.Error = "Limit must be under 100"
        Log.Error("Limit exceeded 100")
        json.NewEncoder(w).Encode(r)
    }
    if(start.Following == nil){
        def := new(bool)
        *def = true
        start.Following = def
    }
    if(start.Rank == nil){
        def := new(string)
        *def = "interest"
        start.Rank = def
    }
    if(start.Replies == nil){
        def := new(bool)
        *def = true
        start.Replies = def
    }
    Log.WithFields(logrus.Fields{"timestamp": start.Timestamp, "limit": start.Limit,
    "Q": start.Q, "un": start.Un, "following": *start.Following}).Info("params")
    //Generating the list of items
    itemList, err := generateList(start, req)
    //itemList, err := searchES(start, req)
    // Error checking the returned list and returning the proper json response.
    if (err == nil) {
        //it worked
        r.Status = "OK"
        r.Items = itemList
    } else {
            r.Status = "error"
            r.Error = err.Error()
    }
    elapsed := time.Since(startTime)
    Log.Info("Search elapsed: " + elapsed.String())
    json.NewEncoder(w).Encode(r)
}

func getFollowingList(username string) ([]string, error){
    client, err := wrappers.NewClient()
    if err != nil {
        return nil, err
    }
    db := client.Database("twitter")
    coll := db.Collection("following")
    filter := bson.NewDocument(bson.EC.String("user", username))
    proj := bson.NewDocument(bson.EC.Int32("_id",0), bson.EC.Int32("user", 0))
    option, err := mongo.Opt.Projection(proj)
    if err != nil {
        return nil,err
    }
    cursor, err := coll.Find(context.Background(), filter, option)
    if err != nil{
        return nil, err
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
        f, err := doc.Lookup("following")
        if err != nil {
            Log.Error(err)
            return nil, err
        }
        fArr = append(fArr, f.Value().StringValue())
    }
    return fArr, nil
}


func searchES(sPoint params, r *http.Request) ([]item.Item, error) {
    if sPoint.Parent != nil && !*sPoint.Replies {
        return nil, nil
    }
    client, err := wrappers.ESClient()
    search := client.Search().Index("twitter.tweets")
    if err != nil {
        Log.Error(err)
        return nil, err
    }
    filter := elastic.NewBoolQuery()
    filter = filter.Filter(elastic.NewRangeQuery("timestamp").Lte(sPoint.Timestamp))
    if sPoint.Q != "" { // Content
        filter = filter.Filter(elastic.NewMatchQuery("content", sPoint.Q))
    }
    if sPoint.Un != "" { // Username
        filter = filter.Filter(elastic.NewTermQuery("username", sPoint.Un))
    }
    if *sPoint.Following {
        /*
        ind := '\"index\" : \"following\"'
        typ := '\"type\" : \"_doc\"'
        filter = filter.Filter(elastic.NewTermsQuery("username"*/
        currUsername, err := getUsername(r)
        if err != nil {
            return nil, err
        }
        folList, err := getFollowingList(currUsername)
        terms := make([]interface{}, len(folList))
        for i, f := range folList {
            terms[i] = f
        }
        if err != nil {
            return nil, err
        }
        filter = filter.Filter(elastic.NewTermsQuery("username", terms...))
    }
    if *sPoint.Rank == "interest" {
        search = search.SortBy(elastic.NewFieldSort("property.likes").Desc(),
            elastic.NewFieldSort("retweeted").Desc())
    } else { // Sort by time.
        search = search.SortBy(elastic.NewFieldSort("timestamp").Asc())
    }
    if sPoint.Parent != nil {
        filter = filter.Filter(elastic.NewTermQuery("parent", *sPoint.Parent))
        filter = filter.Filter(elastic.NewTermQuery("childType", "reply"))
    }
    if !*sPoint.Replies { // Exclude reply items.
        filter = filter.MustNot(elastic.NewTermQuery("childType", "reply"))
    }
    if sPoint.HasMedia { // Exclude items that don't media.
        filter = filter.Filter(elastic.NewExistsQuery("media"))
    }
    // query := elastic.NewConstantScoreQuery(filter)
    query := filter
    src, _ := query.Source()
    strQ, _ := json.Marshal(src)
    Log.Debug(string(strQ))
    search = search.Query(query).
        Size(int(sPoint.Limit))
    searchRes, err := search.Do(context.Background())
    if err != nil {
        Log.Error(err)
        return nil, err
    } else {
        Log.Debug(searchRes)
    }
    var tweet item.Item
    var tweets []item.Item
    for _, it := range searchRes.Each(reflect.TypeOf(tweet)) {
        t := it.(item.Item)
        tweets = append(tweets, t)
    }
    return tweets, err
}

func generateList(sPoint params, r *http.Request) ([]item.Item, error){
    //Connecting to db and setting up the collection
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error("Could not connect to Mongo.")
        return nil, err
    }
    Log.Info(sPoint)
    db := client.Database("twitter")
    col := db.Collection("tweets")

    var tweetList []item.Item
    var info item.Item
    //var prop property
    user,err := getUsername(r)
    if err != nil{
        Log.Error(err)
        return nil,err
    }
    matchFilter := bson.NewDocument()
    matchFilter.Append(bson.EC.SubDocumentFromElements(
            "timestamp",
            bson.EC.Int64("$lte", (int64)(sPoint.Timestamp))))
    if sPoint.Un != "" {
        matchFilter.Append(bson.EC.String("username", sPoint.Un))
    }
    if(*(sPoint.Following) == true){
        followingList, err := getFollowingList(user)
        if err != nil {
            return nil, err
        }
        bArray := bson.NewArray()
        for _,element := range followingList {
            bArray.Append(bson.VC.String(element))
        }
        Log.Debug(bArray)
        matchFilter.Append(
            bson.EC.SubDocumentFromElements("username",
            bson.EC.Array("$in", bArray)),
        )
    }
    if sPoint.Q != "" {
        matchFilter.Append(
            bson.EC.SubDocumentFromElements("$text",
            bson.EC.String("$search", sPoint.Q)))
    }
    if *sPoint.Replies == false {
        // Exclude reply tweets.
        matchFilter.Append(
                bson.EC.SubDocumentFromElements("childType",
                bson.EC.String("$ne", "reply")))

    }
    if sPoint.Parent != nil {
        // Only return tweets where parent = given parentId.
        pOID, err := objectid.FromHex(*sPoint.Parent)
        if err != nil {
                Log.Error("Invalid Parent ID")
                return nil, err
        }
        matchFilter.Append(bson.EC.ObjectID("parent", pOID))
    }
    if sPoint.HasMedia {
        matchFilter.Append(bson.EC.SubDocumentFromElements("media",
            bson.EC.Boolean("$exists", true)))
    }
    match := bson.EC.SubDocument(
        "$match",
        matchFilter)
    pipeline := bson.NewArray()
    pipeline.Append(bson.VC.Document(bson.NewDocument(match)))
    if *sPoint.Rank == "interest" {
        Log.Debug("Interest is the ranking")
        sort := bson.EC.SubDocument(
                "$sort", bson.NewDocument(
        bson.EC.Int32("property.likes", -1),
        bson.EC.Int32("retweeted", -1)))
        pipeline.Append(bson.VC.Document(bson.NewDocument(sort)))
    }
    limit := bson.VC.DocumentFromElements(
        bson.EC.Int64("$limit", sPoint.Limit))
    pipeline.Append(limit)
    Log.Debug(pipeline)
    set, err := col.Aggregate(
            context.Background(),
            pipeline)
    if err != nil {
        Log.Error("Problem with query")
        return nil, err
    } else {
        Log.Debug(set)
        for set.Next(context.Background()) {
            err = set.Decode(&info)
            tweetList = append(tweetList, info)
        }
    }
    if tweetList == nil {
            tweetList = []item.Item{}
    }
    return tweetList, nil
}
