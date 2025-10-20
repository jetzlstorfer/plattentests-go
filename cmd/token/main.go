package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	myauth "github.com/jetzlstorfer/plattentests-go/internal/auth"
	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

const Port = "8080"
const RedirectURI = "http://localhost:" + Port + "/callback"

var (
	SpotifyAuthenticator = spotifyauth.New(spotifyauth.WithRedirectURL(RedirectURI), spotifyauth.WithScopes(spotifyauth.ScopePlaylistModifyPrivate, spotifyauth.ScopePlaylistModifyPublic))
	channel              = make(chan *spotify.Client)
	state                = "myCrazyState"
	config               struct {
		TokenFile string `envconfig:"TOKEN_FILE" required:"true"`
	}
)

func main() {
	// start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":"+myauth.Port, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	url := myauth.SpotifyAuthenticator.AuthURL(state)
	log.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	client := <-channel

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.ID)
	log.Printf("You are logged in as: %s (%s)", user.DisplayName, user.ID)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	token, err := SpotifyAuthenticator.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	// we have a valid token, lets proceed

	btys, err := json.Marshal(token)
	if err != nil {
		log.Fatalf("could not marshal token: %v", err)
	}

	err = os.WriteFile(config.TokenFile, btys, 0644)
	if err != nil {
		log.Fatalf("could not write file: %v", err)
	}

	_, err = myauth.UploadBytesToBlob(btys)
	if err != nil {
		log.Fatalf("Could not upload token: %v", err)
	}
	log.Println("Token uploaded as: " + config.TokenFile)

	// use the token to get an authenticated client
	httpClient := spotifyauth.New().Client(context.Background(), token)
	client := spotify.New(httpClient)
	user, _ := client.CurrentUser(context.Background())
	fmt.Fprintf(w, "Login Completed!\nYou are now logged in as "+user.ID)
	channel <- client
}
