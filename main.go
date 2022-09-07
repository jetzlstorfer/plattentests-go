// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//     - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"

	"golang.org/x/oauth2"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd"

	"github.com/kelseyhightower/envconfig"
	"github.com/zmb3/spotify"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
// const redirectURI = "http://localhost:8888/callback"
const MAX_SEARCH_RESULTS = 3
const MAX_RECORDS_OF_THE_WEEK = 25

var (
	config struct {
		TargetPlaylist  string `envconfig:"PLAYLIST_ID" required:"true"`
		ProdPlaylist    string `envconfig:"PROD_PLAYLIST_ID"`
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
		LogFile         string `envConfig:"LOG_FILE" required:"false"`
	}
)
var (
	version string
	date    string
)

var playlistID spotify.ID

func main() {

	r := gin.Default()
	r.GET("/api/createPlaylist/", handler)
	r.GET("/api/records/", crawler.PrintRecordsOfTheWeek)
	r.GET("/api/records/:id", crawler.GetRecord)
	r.Run(get_port())

}

func handler(c *gin.Context) {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	// probably not used!?
	// if config.LogFile == "true" {
	// 	logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	mw := io.MultiWriter(os.Stdout, logFile)
	// 	log.SetOutput(mw)
	// }

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
	// only use the first records up to MAX_RECORDS_OF_THE_WEEK
	// this is mainly for debugging purposes
	if len(highlights) > MAX_RECORDS_OF_THE_WEEK {
		highlights = highlights[0:MAX_RECORDS_OF_THE_WEEK]
	}

	log.Println("Size of records of the week: ", len(highlights))

	log.Println("---")
	log.Println("Connecting to Spotify")
	log.Println("---")
	//_, _ = verifyLogin()
	client, _ := verifyLogin()

	log.Println("Emptying playlist...")
	client.ReplacePlaylistTracks(playlistID)

	log.Println("Adding highlights of the week to playlist...")
	total := 0
	// var newTracks []spotify.ID
	var notFound []string
	for _, record := range highlights {
		log.Println(record.Band + " - " + record.Recordname + ": " + record.Link)
		var itemsToAdd []spotify.ID
		for _, track := range record.Tracks {

			itemID := searchSong(client, track, record)
			if itemID != "" {
				log.Println("adding item to collection to be added: " + itemID)
				itemsToAdd = append(itemsToAdd, itemID)
				// newTracks = append(newTracks, itemsToAdd...)
			} else {
				notFound = append(notFound, track)
			}
			total++
		}
		// remove duplicates
		noDuplicateTracks := removeDuplicates(itemsToAdd)

		// now add tracks to playlist
		addTracks(client, noDuplicateTracks...)
	}
	log.Println()
	log.Println("--- RESULTS ---")
	log.Println()
	log.Println("total tracks:     ", total)
	log.Println("found tracks:     ", total-len(notFound))
	log.Println("not found tracks: ", len(notFound))
	log.Println()
	log.Println("Not found search terms: ")

	for _, track := range notFound {
		log.Println(" " + track)
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
	// fmt.Println(blobData)

	return blobData.Bytes(), nil
}

func verifyLogin() (spotify.Client, error) {
	log.Println("Connecting to Azure to download token")

	buff, _ := DownloadBlogToBytes("")

	log.Println("Token downloaded from Azure")
	tok := new(oauth2.Token)
	if err := json.Unmarshal(buff, tok); err != nil {
		log.Fatalf("could not unmarshal token: %v", err)
	}

	// Create a Spotify authenticator with the oauth2 token.
	// If the token is expired, the oauth2 package will automatically refresh
	// so the new token is checked against the old one to see if it should be updated.
	log.Println("Creating Spotify Authenticator")
	client := spotify.NewAuthenticator("").NewClient(tok)

	log.Println("Creating new Client Token")
	newToken, err := client.Token()
	if err != nil {
		log.Fatalf("Could not retrieve token from client: %v", err)
	}
	if newToken.AccessToken != tok.AccessToken {
		log.Println("Got refreshed token, saving it")
	}

	_, err = UploadBytesToBlob(buff)
	if err != nil {
		log.Fatalf("Could not upload token: %v", err)
	}

	log.Println("Token uploaded.")

	// use the client to make calls that require authorization
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in as: ", user.ID)

	return client, nil
}

// searches a song given by the track and record name and returns spotify.ID if successful
func searchSong(client spotify.Client, track string, record crawler.Record) spotify.ID {
	searchTerm := sanitizeTrackname(track)
	searchTerm = searchTerm + " " + record.Recordname

	// if record has a year, append it to the search
	if record.ReleaseYear != "" {
		searchTerm += " year:" + record.ReleaseYear
	}

	log.Printf(" searching term: %s", searchTerm)
	results, err := client.Search(searchTerm, spotify.SearchTypeTrack)
	if err != nil {
		log.Fatal(err)
	}
	// handle track results only if tracks are available
	if results.Tracks != nil && results.Tracks.Tracks != nil && len(results.Tracks.Tracks) > 0 {
		allTrackNames := []string{}
		for i, item := range results.Tracks.Tracks {
			log.Printf(" found item: %s - %s  (%s)", item.Artists[0].Name, item.Name, item.Album.Name)
			allTrackNames = append(allTrackNames, item.Name)
			// only get MAX_SEARCH_RESULTS results
			if i >= MAX_SEARCH_RESULTS-1 {
				break
			}
		}
		item := results.Tracks.Tracks[0]

		// do some fuzzy search
		ranking := fuzzy.RankFind(searchTerm, allTrackNames)
		log.Printf("%d %d %+v", len(allTrackNames), len(ranking), ranking)

		if strings.EqualFold(item.Artists[0].Name, record.Band) {
			log.Printf(" using item: %s - %s (%s)", item.Artists[0].Name, item.Name, item.Album.Name)
			return item.ID
		} else {
			log.Printf(" not adding item %s - %s (%s) since artists don't match (%s != %s)", item.Artists[0].Name, item.Name, item.Album.Name, record.Band, item.Artists[0].Name)
			return ""
		}

	}

	if record.Recordname == "" {
		log.Printf(" nothing found for %s", searchTerm)
		return ""
	}
	log.Println(" nothing found, removing recordname and year from search query")
	newRecord := record
	newRecord.ReleaseYear = ""
	newRecord.Recordname = ""
	return searchSong(client, track, newRecord)

}

// adds tracks to global playlist ID
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

// removes parts of string that should not be in search term
func sanitizeTrackname(trackname string) string {
	sanitizedName := trackname
	sanitizedName = strings.Split(sanitizedName, "(feat. ")[0]
	sanitizedName = strings.Split(sanitizedName, "(with ")[0]
	sanitizedName = strings.Split(sanitizedName, "(Bonus)")[0]
	return sanitizedName
}

// remove duplicates from array
func removeDuplicates(sliceList []spotify.ID) []spotify.ID {
	allKeys := make(map[spotify.ID]bool)
	list := []spotify.ID{}
	for _, item := range sliceList {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func get_port() string {
	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	return port
}
