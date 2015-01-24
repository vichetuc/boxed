package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
	"github.com/tejo/dropbox"
)

var store = sessions.NewCookieStore([]byte("182hetsgeih8765$aasdhj"))

type User struct {
	DropboxID string
	Name      string
}

var AppToken = dropbox.AppToken{
	Key:    "2vhv4i5dqyl92l1",
	Secret: "0k1q9zpbt1x3czk",
}

var RequestToken dropbox.RequestToken
var AccessToken dropbox.AccessToken
var callbackUrl = "http://localhost:8080/oauth/callback"

var db *bolt.DB

func init() {
	gob.Register(dropbox.RequestToken{})
}

func main() {
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30 * 12,
		HttpOnly: true,
	}
	var err error
	db, err = bolt.Open("blog.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("UserData"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	router := httprouter.New()
	router.GET("/", Index)
	router.GET("/login", Login)
	router.GET("/oauth/callback", Callback)

	log.Fatal(http.ListenAndServe(":8080", router))
}
func Login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, _ := store.Get(r, "godropblog")
	RequestToken, _ = dropbox.StartAuth(AppToken)
	session.Values["RequestToken"] = RequestToken
	session.Save(r, w)
	url, _ := url.Parse(callbackUrl)
	authUrl := dropbox.GetAuthorizeURL(RequestToken, url)
	http.Redirect(w, r, authUrl.String(), 302)
}

func Callback(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, _ := store.Get(r, "godropblog")
	RequestToken := session.Values["RequestToken"].(dropbox.RequestToken)
	AccessToken, _ = dropbox.FinishAuth(AppToken, RequestToken)
	info, err := dbClient(AccessToken).GetAccountInfo()
	if err != nil {
		log.Println(err)
	}
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("UserData"))
		t, _ := json.Marshal(AccessToken)
		uid := strconv.Itoa(int(info.Uid))
		err := b.Put([]byte(uid+":token"), []byte(t))
		i, _ := json.Marshal(info)
		err = b.Put([]byte(uid), []byte(i))
		return err
	})
	session.Values["key"] = AccessToken.Key
	session.Values["secret"] = AccessToken.Secret
	session.Save(r, w)
	fmt.Printf("AccessToken = %+v\n", AccessToken)
	http.Redirect(w, r, "/", 302)
}

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, _ := store.Get(r, "godropblog")

	if key, secret := session.Values["key"], session.Values["secret"]; key == nil && secret == nil {
		// http.Redirect(w, r, "/login", 302)
		return
	} else {
		AccessToken.Key = key.(string)
		AccessToken.Secret = secret.(string)
	}
	db := dbClient(AccessToken)
	info, err := db.GetAccountInfo()

	if err != nil {
		//access token is not valid anymore
		fmt.Fprintf(w, " %+v\n", err)
		// reset all session
		session.Values["key"], session.Values["secret"] = "", ""
		session.Save(r, w)
		// http.Redirect(w, r, "/login", 302)
		return
	}

	fmt.Printf("err = %+v\n", err)
	fmt.Printf("err = %+v\n", info)
	db.CreateDir("drafts")
	db.CreateDir("published")
	delta, err := db.GetDelta()
	fmt.Printf("delta = %+v\n", delta)
	fmt.Printf("delta err = %+v\n", err)
}

func dbClient(t dropbox.AccessToken) *dropbox.Client {
	return &dropbox.Client{
		AppToken:    AppToken,
		AccessToken: AccessToken,
		Config: dropbox.Config{
			Access: dropbox.AppFolder,
			Locale: "us",
		}}
}
