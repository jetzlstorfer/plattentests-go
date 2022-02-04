// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//       - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"

	"golang.org/x/oauth2"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/jetzlstorfer/plattentests-go/crawler"

	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:8888/callback"
const logFile = "log.txt"

var (
	config struct {
		TargetPlaylist  string `envconfig:"PLAYLIST_ID" required:"true"`
		ProdPlaylist    string `envconfig:"PROD_PLAYLIST_ID"`
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
		LogFile         string `required:"false"`
	}
)
var (
	version string
	date    string
)

var playlistID spotify.ID
var plID = flag.String("playlistid", "", "The id of the playlist to be modified")
var prod = flag.String("prod", "", "Set to true to create production playlist")

func main() {

	handler()

}

func handler() {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	if config.LogFile == "true" {
		logFile, err := os.OpenFile(logFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
		if err != nil {
			panic(err)
		}
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
	}

	playlistID = spotify.ID(os.Getenv("PLAYLIST_ID"))

	log.Println("Plattentests.de Highlights of the week playlist generator")
	log.Printf("version=%s, date=%s\n", version, date)
	log.Println()

	if playlistID == "" || os.Getenv("SPOTIFY_ID") == "" || os.Getenv("SPOTIFY_SECRET") == "" {
		log.Fatalln("PLAYLIST_ID, SPOTIFY_ID, or SPOTIFY_SECRET missing.")
	}

	log.Println("Getting tracks of the week...")
	highlights := crawler.GetRecordsOfTheWeek()

	// sort record collection
	sort.Slice(highlights[:], func(i, j int) bool {
		return highlights[i].Score > highlights[j].Score
	})

	// put record of the week on top
	recordOfTheWeek := crawler.GetRecordOfTheWeek()
	for i, record := range highlights {
		if record.Band == recordOfTheWeek {
			highlights = append(highlights[:i], highlights[i+1:]...)
			highlights = append([]crawler.Record{record}, highlights...)
			break
		}
	}
	log.Println("Size of records of the week: ", len(highlights))

	log.Println("---")
	log.Println("Connecting to Spotify")
	log.Println("---")
	_, _ = verifyLogin()
	client, _ := verifyLogin()

	log.Println("Emptying playlist...")
	client.ReplacePlaylistTracks(playlistID)

	log.Println("Adding highlights of the week to playlist....")
	total := 0
	var newTracks []spotify.ID
	var notFound []string
	for _, record := range highlights {
		log.Println(record.Band + record.Name + ": " + record.Link)
		var itemsToAdd []spotify.ID
		for _, track := range record.Tracks {

			itemID := searchSong(client, track, record)
			if itemID != "" {
				log.Println("adding item to collection to be added: " + itemID)
				itemsToAdd = append(itemsToAdd, itemID)
				newTracks = append(newTracks, itemsToAdd...)
			} else {
				notFound = append(notFound, track)
			}
			total++
		}
		addTracks(client, itemsToAdd...)
	}
	log.Println()
	log.Println("total tracks:     ", total)
	log.Println("found tracks:     ", total-len(notFound))
	log.Println("not found tracks: ", len(notFound))
	log.Println()
	log.Println("Not found search terms: ")

	for _, track := range notFound {
		log.Println(track)
	}

}

func GetAccountInfo() (string, string, string, string) {
	azrKey := config.AzAccountKey
	azrBlobAccountName := config.AzAccountName
	azrPrimaryBlobServiceEndpoint := fmt.Sprintf("https://%s.blob.core.windows.net/", azrBlobAccountName)
	azrBlobContainer := config.AzContainerName

	return azrKey, azrBlobAccountName, azrPrimaryBlobServiceEndpoint, azrBlobContainer
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
	blobData.ReadFrom(reader)
	reader.Close() // The client must close the response body when finished with it
	fmt.Println(blobData)

	return blobData.Bytes(), nil
}

func verifyLogin() (spotify.Client, error) {
	log.Println("Connecting to Azure to download token")

	buff, _ := DownloadBlogToBytes("")

	tok := new(oauth2.Token)
	if err := json.Unmarshal(buff, tok); err != nil {
		log.Fatalf("could not unmarshal token: %v", err)
	}

	// Create a Spotify authenticator with the oauth2 token.
	// If the token is expired, the oauth2 package will automatically refresh
	// so the new token is checked against the old one to see if it should be updated.
	client := spotify.NewAuthenticator("").NewClient(tok)

	newToken, err := client.Token()
	if err != nil {
		log.Fatalf("could not retrieve token from client: %v", err)
	}
	if newToken.AccessToken != tok.AccessToken {
		log.Println("got refreshed token, saving it")
	}

	_, err = UploadBytesToBlob(buff)
	if err != nil {
		log.Fatalf("cound not upload token: %v", err)
	}

	// use the client to make calls that require authorization
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("You are logged in as: ", user.ID)

	return client, nil
}

func searchSong(client spotify.Client, track string, record crawler.Record) spotify.ID {
	searchTerm := sanitizeTrackname(track)
	searchTerm = searchTerm + " " + record.Name
	log.Printf(" searching term: %s", searchTerm)
	results, err := client.Search(searchTerm, spotify.SearchTypeTrack)
	if err != nil {
		log.Fatal(err)
	}
	// handle track results
	if results.Tracks != nil && results.Tracks.Tracks != nil && len(results.Tracks.Tracks) > 0 {
		for i, item := range results.Tracks.Tracks {
			log.Printf(" found item: %s - %s", item.Name, item.Album.Name)
			if i >= 3 {
				break
			}
		}
		item := results.Tracks.Tracks[0]
		log.Printf(" using item: %s - %s", item.Name, item.Album.Name)
		return item.ID

	}

	if record.Name == "" {
		log.Printf(" nothing found for %s", searchTerm)
		return ""
	}
	log.Println(" nothing found, removing recordname from search query")
	return searchSong(client, track, crawler.Record{"", "", "", 0, nil})

}

func addTracks(client spotify.Client, trackids ...spotify.ID) bool {
	if len(trackids) == 0 {
		log.Println("no tracks to add")
		return false
	}
	_, err := client.AddTracksToPlaylist(playlistID, trackids...)
	if err != nil {
		log.Fatalf("could not add tracks to playlist: %s", err)
		return false
	}
	return true

}

func sanitizeTrackname(trackname string) string {
	sanitizedName := trackname
	sanitizedName = strings.Split(sanitizedName, "(feat. ")[0]
	sanitizedName = strings.Split(sanitizedName, "(with ")[0]
	sanitizedName = strings.Split(sanitizedName, "(Bonus)")[0]
	return sanitizedName
}
