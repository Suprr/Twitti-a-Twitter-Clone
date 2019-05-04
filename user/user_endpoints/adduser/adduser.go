package adduser

import (
  "time"
    "context"
    "errors"
    "github.com/sirupsen/logrus"
    "net/http"
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
    "crypto/md5"
    "encoding/hex"
    "net/smtp"
    "math/rand"
    "strconv"
    "TwitterClone/user"
    "TwitterClone/wrappers"
)

type request struct {
    Username *string `json:"username"`
    Password *string `json:"password"`
    Email *string `json:"email"`
}

type response struct {
    Status string `json:"status"`
    Error string `json:"error,omitempty"`
}
var Log *logrus.Logger

func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func checkUserExists(username string, email string) (error) {
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error(err)
        return err
    }
    db := client.Database("twitter")
    coll := db.Collection("emails")
    filter := bson.NewDocument(bson.EC.String("email", email))
    result := bson.NewDocument()
    err = coll.FindOne(context.Background(), filter).Decode(result)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint": "adduser","msg":"email FindOne time elapsed",
        "timeElapsed":elapsed.String()}).Info()
    if err == nil { // Email exists.
        err = errors.New("The email " + email + " is already in use.")
        Log.Error(err)
        return err
    }
    // If error is not nil, this probably means it wasn't found.
    // However, it's possible it's an actual error, so it's being logged.
    Log.WithFields(logrus.Fields{"endpoint":"adduser", "msg": "email FindOne error",
        "error": err.Error()}).Debug()

    coll = db.Collection("usernames")
    filter = bson.NewDocument(bson.EC.String("username", username))
    dbStart = time.Now()
    result = bson.NewDocument()
    err = coll.FindOne(context.Background(), filter).Decode(result)
    elapsed = time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint": "adduser","msg":"username FindOne time elapsed",
        "timeElapsed":elapsed.String()}).Info()
    if err == nil { // Username exists.
        err = errors.New("The username " + username + " is already in use.")
        Log.Error(err)
        return err
    }
    // If error is not nil, this probably means it wasn't found.
    // However, it's possible it's an actual error, so it's being logged.
    Log.WithFields(logrus.Fields{"endpoint":"adduser", "msg": "username FindOne error",
        "error": err.Error()}).Debug()
    return nil
}

func insertUser(username string, email string, password string) (string, error) {
    dbStart := time.Now()
    client, err := wrappers.NewClient()
    if err != nil {
        Log.Error(err)
        return "", err
    }
    db := client.Database("twitter")
    hashedPassword := md5.Sum([]byte(password))
    var user user.User
    user.Username = username
    user.Email = email
    user.Password = hashedPassword[:]
    user.Verified = false
    oid := objectid.New()
    user.ID = oid
    Log.Debug(oid)
    // Create the hashed verification key.
    num := rand.Int()
    numstring := strconv.Itoa(num)
    Log.Println(num, numstring)
    hasher := md5.New()
    hasher.Write([]byte(user.Username))
    hasher.Write([]byte(numstring))
    key := hex.EncodeToString(hasher.Sum(nil))
    key = "<" + key + ">"
    user.Key = key
    dbStart = time.Now()
    // Update users collection.
    Log.Debug(user)
    coll := db.Collection("users")
    _, err = coll.InsertOne(context.Background(), &user)
    elapsed := time.Since(dbStart)
    Log.WithFields(logrus.Fields{"endpoint": "adduser",
            "msg":"insert a user time elapsed",
            "timeElapsed":elapsed.String()}).Info()
    if err != nil {
        Log.Error(err)
        return "", err
    }
    // Update usernames collection.
    coll = db.Collection("usernames")
    doc := bson.NewDocument(
        bson.EC.String("username", user.Username),
        bson.EC.ObjectID("_id", user.ID))
    _, err = coll.InsertOne(context.Background(), doc)
    if err != nil {
        Log.Error(err)
        return "", err
    }
    // Update emails collection.
    coll = db.Collection("emails")
    doc = bson.NewDocument(
        bson.EC.String("email", user.Email),
        bson.EC.ObjectID("_id", user.ID))
    _, err = coll.InsertOne(context.Background(), doc)
    if err != nil {
        Log.Error(err)
        return "", err
    } else {
        return key, nil
     }
}

func sendError(w http.ResponseWriter, err error) {
    var res response
    res.Status = "error"
    res.Error = err.Error()
    encodeResponse(w, res)
}

func AddUserHandler(w http.ResponseWriter, req *http.Request) {
    Log.SetLevel(logrus.InfoLevel)
    start := time.Now()
    decoder := json.NewDecoder(req.Body)
    var us request
    err := decoder.Decode(&us)
    if err != nil {
        sendError(w, err)
        return
    }
    err = validateUser(us)
    if err != nil {
        sendError(w, err)
        return
    }
    // Add the user.
    err = checkUserExists(*us.Username, *us.Email)
    if err != nil {
        Log.Error(err)
        sendError(w, err)
        return
    }
    var res response
    // No error. Add user to db and send email.

    key, err := insertUser(*us.Username, *us.Email, *us.Password)
    if err != nil {
        Log.Error(err)
        sendError(w, err)
        return
    }else {
        res.Status = "OK"
        encodeResponse(w, res)
        elapsed := time.Since(start)
        Log.Info("Add User elapsed: " + elapsed.String())
        go sendEmail(*us.Email, key)
    }
}

func sendEmail(email string, key string) error {
    link := "http://nsamba.cse356.compas.cs.stonybrook.edu/verify?email="+email+"&key="+key
    msg := []byte("To: "+email+"\r\n" +
    "From:  mongo-config@cloud.compas.cs.stonybrook.edu" + "\r\n" +
    "Subject: Validation Email\r\n" +
    "\r\n" +
    "Thank you for joining Twiti!\n This is your validation key: " + key + "\n Please click the link to quickly veify your account: "+ link+"\r\n")
    addr := "192.168.1.24:25"
    err := smtp.SendMail(addr, nil,
    "<mongo-config>",
    []string{email}, msg)
    if err != nil {
        Log.Error(err)
    }
    return err
}

func validateUser(us request) error {
    var err error
    if (us.Username == nil) {
        err = errors.New("No username in adduser request.")
    } else if (us.Password == nil) {
        err = errors.New("No password in adduser request.")
    }else if (us.Email == nil) {
        err = errors.New("No email in adduser request.")
    }
    if err != nil {
        Log.Error(err)
    }
    return err
}
