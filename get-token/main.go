package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const port = "8080"
const redirectURI = "http://localhost:" + port + "/callback"

var (
	auth    = spotifyauth.New(spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(spotifyauth.ScopePlaylistModifyPrivate))
	channel = make(chan *spotify.Client)
	state   = "myCrazyState"
	config  struct {
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
	}
)

func main() {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal("could not process env variables: " + err.Error())
	}

	// start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	url := auth.AuthURL(state)
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
	token, err := auth.Token(r.Context(), state, r)
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

	err = ioutil.WriteFile(config.TokenFile, btys, 0644)
	if err != nil {
		log.Fatalf("could not write file: %v", err)
	}

	_, err = UploadBytesToBlob(btys)
	if err != nil {
		log.Fatalf("Could not upload token: %v", err)
	}

	log.Println("Token uploaded.")

	// use the token to get an authenticated client
	client := spotify.New(auth.Client(r.Context(), token))
	fmt.Fprintf(w, "Login Completed!")
	channel <- client
}

func UploadBytesToBlob(b []byte) (string, error) {
	azrKey, accountName, endPoint, container := GetAccountInfo()
	u, _ := url.Parse(fmt.Sprint(endPoint, container, "/", config.TokenFile))
	credential, errC := azblob.NewSharedKeyCredential(accountName, azrKey)
	if errC != nil {
		return "", errC
	}

	blockBlobUrl := azblob.NewBlockBlobURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{}))

	ctx := context.Background()
	o := azblob.UploadToBlockBlobOptions{
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			ContentType: "application/json",
		},
	}

	_, errU := azblob.UploadBufferToBlockBlob(ctx, b, blockBlobUrl, o)
	return blockBlobUrl.String(), errU
}

func GetAccountInfo() (string, string, string, string) {
	azrKey := config.AzAccountKey
	azrBlobAccountName := config.AzAccountName
	azrPrimaryBlobServiceEndpoint := fmt.Sprintf("https://%s.blob.core.windows.net/", azrBlobAccountName)
	azrBlobContainer := config.AzContainerName

	return azrKey, azrBlobAccountName, azrPrimaryBlobServiceEndpoint, azrBlobContainer
}
