package main

import (
    "net/http"
    "github.com/gorilla/mux"
    "TwitterClone/additem"
    "TwitterClone/user/user_endpoints"
    "TwitterClone/user/user_endpoints/followInfo"
    "TwitterClone/user/user_endpoints/adduser"
    "TwitterClone/search"
    "TwitterClone/media/addmedia"
    "TwitterClone/media/media_endpoints"
    "TwitterClone/follow"
    "TwitterClone/item/item_endpoints"
    "TwitterClone/verify"
    "TwitterClone/login"
    "TwitterClone/logout"
)

type Route struct {
    Name string
    Method string
    Path string
    HandlerFunc http.HandlerFunc
}

var routes = []Route{
    Route {
        "additem",
        "POST",
        "/additem",
        additem.AddItemHandler},
    Route {
        "adduser",
        "POST",
        "/adduser",
        adduser.AddUserHandler},
    Route {
        "login",
        "POST",
        "/login",
        login.LoginHandler},
    Route {
        "logout",
        "POST",
        "/logout",
        logout.LogoutHandler},
    Route {
        "verify",
        "POST",
        "/verify",
        verify.VerifyHandler},
    Route {
        "GetItem",
        "GET",
        "/item/{id}",
        item_endpoints.GetItemHandler},
    Route {
        "LikeItem",
        "POST",
        "/item/{id}/like",
        item_endpoints.LikeItemHandler},
    Route {
        "DeleteItem",
        "DELETE",
        "/item/{id}",
        item_endpoints.DeleteItemHandler},
    Route {
        "search",
        "POST",
        "/search",
        search.SearchHandler},
    Route {
        "GetUser",
        "GET",
        "/user/{username}",
        user_endpoints.GetUserHandler},
    Route {
        "GetUserFollowers",
        "GET",
        "/user/{username}/followers",
        followInfo.GetFollowersHandler},
    Route {
        "GetUserFollowing",
        "GET",
        "/user/{username}/following",
        followInfo.GetFollowingHandler},
    Route {
        "Follow",
        "POST",
        "/follow",
        follow.FollowHandler},
    Route {
        "AddMedia",
        "POST",
        "/addmedia",
        addmedia.AddMediaHandler},
    Route {
        "GetMedia",
        "GET",
        "/media/{id}",
        media_endpoints.GetMediaHandler},
}

func NewRouter() *mux.Router {
    router := mux.NewRouter()
    for _, route := range routes {
        router.
        Methods(route.Method).
        Path(route.Path).
        Name(route.Name).
        HandlerFunc(route.HandlerFunc)
    }
    return router
}
