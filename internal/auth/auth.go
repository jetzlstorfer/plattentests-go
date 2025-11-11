package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/jetzlstorfer/plattentests-go/internal/secrets"
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
		TokenFile string `envconfig:"TOKEN_FILE" required:"true"`
	}
)

func VerifyLogin() spotify.Client {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Connecting to Azure to download token")

	buff, err := DownloadBlogToBytes("")
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

func DownloadBlogToBytes(string) ([]byte, error) {
	azrKey, accountName, endPoint, container := GetAccountInfo()
	u, _ := url.Parse(fmt.Sprint(endPoint, container, "/", config.TokenFile))
	credential, errC := azblob.NewSharedKeyCredential(accountName, azrKey)
	if errC != nil {
		return nil, errC
	}

	ctx := context.Background()
	blockBlobUrl := azblob.NewBlockBlobURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{}))
	get, err := blockBlobUrl.Download(ctx, 0, 0, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		log.Fatal(err)
	}
	blobData := &bytes.Buffer{}
	reader := get.Body(azblob.RetryReaderOptions{})
	_, err = blobData.ReadFrom(reader)
	if err != nil {
		log.Fatal(err)
	}
	reader.Close() // The client must close the response body when finished with it
	// fmt.Println(blobData)

	return blobData.Bytes(), nil
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
	// making sure all config variables are set
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	azrKey := secrets.GetSecretWithFallback("AZ-KEY", "AZ_KEY")
	azrBlobAccountName := secrets.GetSecretWithFallback("AZ-ACCOUNT", "AZ_ACCOUNT")
	azrPrimaryBlobServiceEndpoint := fmt.Sprintf("https://%s.blob.core.windows.net/", azrBlobAccountName)
	azrBlobContainer := secrets.GetSecretWithFallback("AZ-CONTAINER", "AZ_CONTAINER")

	return azrKey, azrBlobAccountName, azrPrimaryBlobServiceEndpoint, azrBlobContainer
}
