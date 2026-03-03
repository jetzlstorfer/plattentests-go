package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	myauth "github.com/jetzlstorfer/plattentests-go/internal/auth"
	"github.com/jetzlstorfer/plattentests-go/internal/logging"
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
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
	}
)

func main() {
	// Initialize OpenTelemetry logging
	if err := logging.Init(); err != nil {
		logging.Fatal("Failed to initialize logging: %v", err)
	}
	defer logging.Shutdown()

	// Initialize OpenTelemetry tracing
	if err := logging.InitTracing("plattentests-token"); err != nil {
		logging.Fatal("Failed to initialize tracing: %v", err)
	}
	defer logging.ShutdownTracing()

	// start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logging.Info("Got request for: %s", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":"+myauth.Port, nil)
		if err != nil {
			logging.Fatal("Failed to start HTTP server: %v", err)
		}
	}()

	url := myauth.SpotifyAuthenticator.AuthURL(state)
	logging.Info("Please log in to Spotify by visiting the following page in your browser: %s", url)

	// wait for auth to complete
	client := <-channel

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		logging.Fatal("Failed to get current user: %v", err)
	}
	fmt.Println("You are logged in as:", user.ID)
	logging.Info("You are logged in as: %s (%s)", user.DisplayName, user.ID)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	err := envconfig.Process("", &config)
	if err != nil {
		logging.Fatal("Failed to process environment config: %v", err)
	}

	token, err := SpotifyAuthenticator.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		logging.Fatal("Failed to get token: %v", err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		logging.Fatal("State mismatch: %s != %s\n", st, state)
	}

	// we have a valid token, lets proceed

	btys, err := json.Marshal(token)
	if err != nil {
		logging.Fatal("could not marshal token: %v", err)
	}

	err = os.WriteFile(config.TokenFile, btys, 0644)
	if err != nil {
		logging.Fatal("could not write file: %v", err)
	}

	_, err = myauth.UploadBytesToBlob(btys)
	if err != nil {
		logging.Fatal("Could not upload token: %v", err)
	}
	logging.Info("Token uploaded as: %s", config.TokenFile)

	// use the token to get an authenticated client
	httpClient := spotifyauth.New().Client(context.Background(), token)
	client := spotify.New(httpClient)
	user, _ := client.CurrentUser(context.Background())
	fmt.Fprintf(w, "Login Completed!\nYou are now logged in as %s", user.ID)
	channel <- client
}
