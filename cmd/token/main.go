package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	myauth "github.com/jetzlstorfer/plattentests-go/internal/auth"
	"github.com/jetzlstorfer/plattentests-go/internal/observability"
	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

const Port = "8081"
const RedirectURI = "http://127.0.0.1:" + Port + "/callback"

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
	observability.ConfigureLogging("plattentests-token")

	// start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "token manager request", "path", r.URL.Path)
	})
	go func() {
		err := http.ListenAndServe(":"+myauth.Port, nil)
		if err != nil {
			slog.Error("token callback server failed", "error", err)
			os.Exit(1)
		}
	}()

	url := myauth.SpotifyAuthenticator.AuthURL(state)
	slog.Info("visit the Spotify login page", "url", url)

	// wait for auth to complete
	client := <-channel

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		slog.Error("failed to retrieve Spotify user", "error", err)
		os.Exit(1)
	}
	fmt.Println("You are logged in as:", user.ID)
	slog.Info("authenticated Spotify user", "user_id", user.ID)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	err := envconfig.Process("", &config)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to load token configuration", "error", err)
		http.Error(w, "Token configuration is unavailable", http.StatusInternalServerError)
		return
	}

	token, err := SpotifyAuthenticator.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		slog.ErrorContext(r.Context(), "failed to complete Spotify authorization", "error", err)
		return
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		slog.WarnContext(r.Context(), "Spotify authorization state mismatch")
		return
	}

	// we have a valid token, lets proceed

	btys, err := json.Marshal(token)
	if err != nil {
		slog.ErrorContext(r.Context(), "could not marshal token", "error", err)
		http.Error(w, "Could not save token", http.StatusInternalServerError)
		return
	}

	err = os.WriteFile(config.TokenFile, btys, 0644)
	if err != nil {
		slog.ErrorContext(r.Context(), "could not write token file", "error", err)
		http.Error(w, "Could not save token", http.StatusInternalServerError)
		return
	}

	_, err = myauth.UploadBytesToBlob(btys)
	if err != nil {
		slog.ErrorContext(r.Context(), "could not upload token", "error", err)
		http.Error(w, "Could not save token", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(r.Context(), "token uploaded", "token_file", config.TokenFile)

	// use the token to get an authenticated client
	httpClient := spotifyauth.New().Client(context.Background(), token)
	client := spotify.New(httpClient)
	user, _ := client.CurrentUser(context.Background())
	_, _ = fmt.Fprintf(w, "Login Completed!\nYou are now logged in as %s", user.ID)
	channel <- client
}
