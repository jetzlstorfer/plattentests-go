package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"

	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const Port = "8080"
const RedirectURI = "http://localhost:" + Port + "/callback"

var (
	SpotifyAuthenticator = spotifyauth.New(spotifyauth.WithRedirectURL(RedirectURI), spotifyauth.WithScopes(spotifyauth.ScopePlaylistModifyPrivate, spotifyauth.ScopePlaylistModifyPublic))
	config               struct {
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
	}
)

func VerifyLogin() spotify.Client {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Connecting to Azure to download token")

	buff, err := DownloadBlobToBytes("")
	if err != nil {
		log.Fatalf("Could not download token from Azure: %v", err)
	}

	log.Println("Token downloaded from Azure")
	token := new(oauth2.Token)
	if err := json.Unmarshal(buff, token); err != nil {
		log.Fatalf("could not unmarshal token: %v", err)
	}

	// Create a Spotify authenticator with the oauth2 token.
	// If the token is expired, the oauth2 package will automatically refresh
	// so the new token is checked against the old one to see if it should be updated.
	log.Println("Creating Spotify Authenticator")
	ctx := context.Background()
	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	log.Println("Creating new Client Token")
	newToken, err := client.Token()
	if err != nil {
		log.Fatalf("Could not retrieve token from client: %v", err)
	}
	if newToken.AccessToken != token.AccessToken {
		log.Println("Got refreshed token, saving it")
	}

	_, err = UploadBytesToBlob(buff)
	if err != nil {
		log.Fatalf("Could not upload token: %v", err)
	}

	log.Println("Token uploaded.")

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(ctx)
	if err != nil {
		log.Fatalf("Could not identify as user: %v", err)
	}
	log.Printf("Logged in as: %v", user.ID)

	return *client
}

func DownloadBlobToBytes(string) ([]byte, error) {
	azrKey, accountName, _, container := GetAccountInfo()
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)

	cred, err := azblob.NewSharedKeyCredential(accountName, azrKey)
	if err != nil {
		return nil, err
	}

	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	response, err := client.DownloadStream(ctx, container, config.TokenFile, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	blobData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return blobData, nil
}

func UploadBytesToBlob(b []byte) (string, error) {
	azrKey, accountName, _, container := GetAccountInfo()
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)

	cred, err := azblob.NewSharedKeyCredential(accountName, azrKey)
	if err != nil {
		return "", err
	}

	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	blobURL := fmt.Sprintf("%s/%s/%s", serviceURL, container, config.TokenFile)

	reader := bytes.NewReader(b)
	_, err = client.UploadStream(ctx, container, config.TokenFile, reader, nil)

	return blobURL, err
}

func GetAccountInfo() (string, string, string, string) {
	// making sure all config variables are set
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	azrKey := config.AzAccountKey
	azrBlobAccountName := config.AzAccountName
	azrPrimaryBlobServiceEndpoint := fmt.Sprintf("https://%s.blob.core.windows.net/", azrBlobAccountName)
	azrBlobContainer := config.AzContainerName

	return azrKey, azrBlobAccountName, azrPrimaryBlobServiceEndpoint, azrBlobContainer
}
