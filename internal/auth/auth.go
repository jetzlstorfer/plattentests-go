package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"

	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

// Port is the local callback server port used for interactive token setup.
const Port = "8080"

// RedirectURI is the OAuth redirect URI registered in Spotify Developer settings.
const RedirectURI = "http://localhost:" + Port + "/callback"

var (
	// SpotifyAuthenticator performs Spotify OAuth flows for playlist scopes.
	SpotifyAuthenticator = spotifyauth.New(spotifyauth.WithRedirectURL(RedirectURI), spotifyauth.WithScopes(spotifyauth.ScopePlaylistModifyPrivate, spotifyauth.ScopePlaylistModifyPublic))
	config               struct {
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
	}
)

// VerifyLogin downloads the persisted token, refreshes it if needed, uploads it back, and returns an authenticated Spotify client.
func VerifyLogin() (spotify.Client, error) {
	err := envconfig.Process("", &config)
	if err != nil {
		return spotify.Client{}, fmt.Errorf("load auth config: %w", err)
	}

	log.Println("Connecting to Azure to download token")

	buff, err := DownloadBlobToBytes("")
	if err != nil {
		return spotify.Client{}, fmt.Errorf("download token from Azure: %w", err)
	}

	log.Println("Token downloaded from Azure")
	token := new(oauth2.Token)
	if err := json.Unmarshal(buff, token); err != nil {
		return spotify.Client{}, fmt.Errorf("unmarshal token: %w", err)
	}

	// Create a Spotify authenticator with the oauth2 token.
	// If the token is expired, the oauth2 package will automatically refresh
	// so the new token is checked against the old one to see if it should be updated.
	log.Println("Creating Spotify Authenticator")
	ctx := context.Background()
	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	log.Println("Creating new Client Token")
	newToken, err := retrieveTokenWithRetry(client)
	if err != nil {
		return spotify.Client{}, fmt.Errorf("retrieve token from client: %w", err)
	}
	if newToken.AccessToken != token.AccessToken {
		log.Println("Got refreshed token, saving it")
	}

	persistedToken, err := json.Marshal(newToken)
	if err != nil {
		return spotify.Client{}, fmt.Errorf("marshal refreshed token: %w", err)
	}

	_, err = UploadBytesToBlob(persistedToken)
	if err != nil {
		return spotify.Client{}, fmt.Errorf("upload token to Azure: %w", err)
	}

	log.Println("Token uploaded.")

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return spotify.Client{}, fmt.Errorf("identify current user: %w", err)
	}
	log.Printf("Logged in as: %v", user.ID)

	return *client, nil
}

func retrieveTokenWithRetry(client *spotify.Client) (*oauth2.Token, error) {
	const maxAttempts = 4
	const baseDelay = 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := client.Token()
		if err == nil {
			return token, nil
		}

		lastErr = err
		if !isRetryableOAuthError(err) || attempt == maxAttempts {
			break
		}

		delay := time.Duration(attempt) * baseDelay
		log.Printf("temporary Spotify OAuth error while retrieving token (attempt %d/%d): %v; retrying in %s", attempt, maxAttempts, err, delay)
		time.Sleep(delay)
	}

	return nil, lastErr
}

func isRetryableOAuthError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "temporarily_unavailable") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "server error")
}

func isRetryableAzureBlobError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "temporary failure in name resolution") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset")
}

func retryDelay(attempt int, baseDelay time.Duration) time.Duration {
	const maxDelay = 30 * time.Second
	delay := baseDelay * time.Duration(1<<(attempt-1))
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// DownloadBlobToBytes reads the configured token blob from Azure Storage.
func DownloadBlobToBytes(string) ([]byte, error) {
	const maxAttempts = 4
	const baseDelay = 2 * time.Second

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
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		response, err := client.DownloadStream(ctx, container, config.TokenFile, nil)
		if err == nil {
			blobData, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if closeErr != nil {
				log.Printf("failed closing blob download response body: %v", closeErr)
			}
			if readErr == nil {
				return blobData, nil
			}
			err = readErr
		}

		lastErr = err
		if !isRetryableAzureBlobError(err) || attempt == maxAttempts {
			break
		}

		delay := retryDelay(attempt, baseDelay)
		log.Printf("temporary Azure Blob error while downloading token (attempt %d/%d): %v; retrying in %s", attempt, maxAttempts, err, delay)
		time.Sleep(delay)
	}

	return nil, lastErr
}

// UploadBytesToBlob writes bytes to the configured token blob in Azure Storage.
func UploadBytesToBlob(b []byte) (string, error) {
	const maxAttempts = 4
	const baseDelay = 2 * time.Second

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

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		reader := bytes.NewReader(b)
		_, err = client.UploadStream(ctx, container, config.TokenFile, reader, nil)
		if err == nil {
			return blobURL, nil
		}

		lastErr = err
		if !isRetryableAzureBlobError(err) || attempt == maxAttempts {
			break
		}

		delay := retryDelay(attempt, baseDelay)
		log.Printf("temporary Azure Blob error while uploading token (attempt %d/%d): %v; retrying in %s", attempt, maxAttempts, err, delay)
		time.Sleep(delay)
	}

	return blobURL, lastErr
}

// GetAccountInfo returns Azure Storage credentials and endpoints from environment configuration.
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
