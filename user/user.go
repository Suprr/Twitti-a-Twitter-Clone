package user

import (
    "github.com/mongodb/mongo-go-driver/bson/objectid"
)


type User struct {
    ID objectid.ObjectID `json:"id" bson:"_id,omitempty"`
    Username string `json:"username,omitempty" bson:"username"`
    Email string `json:"email,omitempty" bson:"email"`
    Password []byte `json:"password,omitempty" bson:"password"`
    FollowerCount int `json:"followerCount" bson:"followerCount"`
    FollowingCount int `json:"followingCount" bson:"followingCount"`
    Verified bool `json:"verified,omitempty" bson:"verified"`
    Key string `json:"key,omitempty" bson:"key"`
}
