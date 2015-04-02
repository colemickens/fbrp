package main

import (
	"encoding/json"
	"errors"
	"flag"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type FbrpConfig struct {
	AppId         string `json:"app_id"`
	AppSecret     string `json:"app_secret"`
	Hostname      string `json:"hostname"`
	SecretGroupId string `json:"secret_group_id"`
	ServeRoot     string `json:"serve_root"`
	InternalPort  int    `json:"internal_port"`
	SessionSecret string `json:"session_secret"`
}

var CONFIG FbrpConfig

var store *sessions.CookieStore
var SESSION_NAME = "session-name"
var COOKIE_HAS_AUTH = "hasAuth"

var FACEBOOK_OAUTH_CONFIG oauth2.Config
var FACEBOOK_AUTH_CALLBACK_ROUTE = "/auth/login/facebook/callback"

func init() {
	var configFile = flag.String("config", "./fbrp.config", "config file location")
	flag.Parse()
	log.Println("reading config from:", *configFile)
	file, err := os.Open(*configFile)
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(file)
	decoder.Decode(&CONFIG)

	log.Println(CONFIG)

	if CONFIG.SessionSecret == "" {
		panic("SessionSecret should never be empty")
	}
	store = sessions.NewCookieStore([]byte(CONFIG.SessionSecret))

	FACEBOOK_OAUTH_CONFIG = oauth2.Config{
		ClientID:     CONFIG.AppId,
		ClientSecret: CONFIG.AppSecret,
		RedirectURL:  "http://" + CONFIG.Hostname + FACEBOOK_AUTH_CALLBACK_ROUTE,
		Scopes:       []string{"user_about_me", "user_groups"},
		Endpoint:     facebook.Endpoint,
	}
}

func handleFiles(prefix string) http.Handler {
	fs := http.FileServer(http.Dir(CONFIG.ServeRoot))
	return http.StripPrefix(prefix, fs)
}

func requireAuth(innerHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, SESSION_NAME)
		v, ok := session.Values[COOKIE_HAS_AUTH]
		if ok && v.(bool) {
			innerHandler.ServeHTTP(w, r)
		} else {
			w.WriteHeader(403)
			serveString("You're not logged in, login first").ServeHTTP(w, r)
		}
	})
}

func promptFacebookLogin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		facebookLoginUrl := FACEBOOK_OAUTH_CONFIG.AuthCodeURL("login")
		http.Redirect(w, r, facebookLoginUrl, 301)
	})
}

func checkFacebookGroups(token *oauth2.Token) (bool, error) {
	client := FACEBOOK_OAUTH_CONFIG.Client(oauth2.NoContext, token)
	resp, err := client.Get("https://graph.facebook.com/v2.3/me?fields=id,name,groups{id}")
	if err != nil {
		log.Println("failed to check groups")
		log.Println(err)
		return false, err
	}
	defer resp.Body.Close()

	var respData map[string]interface{}

	_ = json.NewDecoder(resp.Body).Decode(&respData)

	name := respData["name"].(string)
	listOfGroups := respData["groups"].(map[string]interface{})
	listOfGroups2 := listOfGroups["data"].([]interface{})

	for _, group := range listOfGroups2 {
		groupId := group.(map[string]interface{})["id"].(string)
		if groupId == CONFIG.SecretGroupId {
			log.Println("LOGIN: ", name)
			return true, nil
		}
	}
	return false, errors.New("You don't belong to the secret group, sorry!")
}

func handleFacebookAuth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		token, _ := FACEBOOK_OAUTH_CONFIG.Exchange(oauth2.NoContext, code)

		userAllowed, err := checkFacebookGroups(token)
		if err != nil {
			serveString("something bad happened: "+err.Error()).ServeHTTP(w, r)
		}

		if userAllowed {
			login(w, r)
		} else {
			logout(w, r)
		}

		http.Redirect(w, r, "/", 301)
	})
}

func handleLogout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logout(w, r)
		http.Redirect(w, r, "/", 301)
	})
}

func serveString(message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isLoggedInStr := "yes!"
		if !isLoggedIn(r) {
			isLoggedInStr = "No!"
		}
		contents := strings.Replace(CONTENTS, "[[[LOGGED IN]]]", isLoggedInStr, 1)
		contents = strings.Replace(contents, "[[[MESSAGE]]]", message, 1)
		w.Write([]byte(contents))
	})
}

func login(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)
	session.Values[COOKIE_HAS_AUTH] = true
	session.Save(r, w)
}

func logout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)
	session.Values[COOKIE_HAS_AUTH] = false
	session.Save(r, w)
}

func isLoggedIn(r *http.Request) bool {
	session, _ := store.Get(r, SESSION_NAME)
	authd, ok := session.Values[COOKIE_HAS_AUTH]
	if ok && authd.(bool) {
		return true
	}
	return false
}

func main() {
	http.Handle(FACEBOOK_AUTH_CALLBACK_ROUTE, handleFacebookAuth())
	http.Handle("/auth/login", promptFacebookLogin())
	http.Handle("/auth/logout", handleLogout())

	FILES_PREFIX := "/files/"
	http.Handle(FILES_PREFIX, requireAuth(handleFiles(FILES_PREFIX)))

	http.Handle("/", serveString(""))

	err := http.ListenAndServe(":"+strconv.Itoa(CONFIG.InternalPort), context.ClearHandler(http.DefaultServeMux))
	if err != nil {
		panic(err)
	}
}

const CONTENTS = `
<html>
<head></head>
<body>
<p>Message: [[[MESSAGE]]]</p>
<p>Logged in? [[[LOGGED IN]]] </p>
<ul>
<li><a href="/auth/login">login</a><br/></li>
<li><a href="/auth/logout">logout</a></li>
<li><a href="/files">files</a><br/></li>
</ul>
</body>
</html>
`
