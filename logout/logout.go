package logout

import (
  "time"
    "net/http"
    "encoding/json"
    logrus "github.com/sirupsen/logrus"
)


type response struct {
    Status string `json:"status"`
    Error string `json:"error,omitempty"`
}
var Log *logrus.Logger
func main() {
    Log.SetLevel(logrus.ErrorLevel)
}

func isLoggedIn(r *http.Request) bool {
    cookie, _ := r.Cookie("username")
    return cookie != nil
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
    return json.NewEncoder(w).Encode(response)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
  start := time.Now()
    cookie, res := logoutEndpoint(r)
    if (cookie != nil) {
        http.SetCookie(w, cookie)
    }

    elapsed := time.Since(start)
    Log.Info("Log out elapsed: " + elapsed.String())
    encodeResponse(w, res)
}

func logoutEndpoint(r *http.Request) (*http.Cookie, response) {
    var res response
    var cookie *http.Cookie
    if isLoggedIn(r) {
        cookie, _ = r.Cookie("username")
        cookie.MaxAge = -1 // Delete cookie.
        res.Status = "OK"
    } else {
        res.Status = "error"
        res.Error = "User not logged in."
    }
    return cookie, res
}
