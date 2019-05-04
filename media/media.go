package media

import(
    "encoding/json"
    "github.com/mongodb/mongo-go-driver/bson/objectid"
)

type Media struct {
    ID objectid.ObjectID `bson:"_id"`
    ContentType string `bson:"contentType"`
    Content []byte `bson:"content"`
    Username string `bson:"username"`
    ItemIDs []objectid.ObjectID `bson:"item_ids,omitempty"`
}

type internalMedia struct {
    ID string `bson:"_id"`
    ContentType string `bson:"contentType"`
    Content []byte `bson:"content"`
    Username string `bson:"username"`
    ItemIDs []string `bson:"item_ids,omitempty"`
}

func (m Media) MarshalJSON() ([]byte, error) {
    var inMedia internalMedia
    oid := m.ID.Hex()
    inMedia.ID = oid
    for _, itOID := range m.ItemIDs {
        itID := itOID.Hex()
        inMedia.ItemIDs = append(inMedia.ItemIDs, itID)
    }
    inMedia.ContentType = m.ContentType
    inMedia.Content = m.Content
    inMedia.Username = m.Username
    return json.Marshal(inMedia)
}
func (m *Media) UnmarshalJSON(b []byte) error {
    var inMedia internalMedia
    err := json.Unmarshal(b, &inMedia)
    if err != nil {
        return err
    }
    oid, err := objectid.FromHex(inMedia.ID)
    if err != nil {
        return err
    }
    for _, itID := range inMedia.ItemIDs {
        itOID, err := objectid.FromHex(itID)
        if err != nil {
            return err
        }
        m.ItemIDs = append(m.ItemIDs, itOID)
    }
    m.ID = oid
    m.ContentType = inMedia.ContentType
    m.Content = inMedia.Content
    m.Username = inMedia.Username
    return nil
}

func CacheKey(id string) string {
    return "media_" + id
}
