// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//     - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	myauth "github.com/jetzlstorfer/plattentests-go/internal/auth"
	"github.com/zmb3/spotify/v2"

	"github.com/kelseyhightower/envconfig"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

const MAX_SEARCH_RESULTS = 3
const MAX_RECORDS_OF_THE_WEEK = 25

var (
	config struct {
		TargetPlaylist  string `envconfig:"PLAYLIST_ID" required:"true"`
		ProdPlaylist    string `envconfig:"PROD_PLAYLIST_ID"`
		LogFile         string `envConfig:"LOG_FILE" required:"false"`
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
	}
)

var playlistID spotify.ID

func main() {
	// log a welcome message in bold letters
	log.Println("\033[1mPlattentests.de Highlights of the week playlist generator\033[0m")

	r := gin.Default()
	r.GET("/api/createPlaylist/", handler)
	r.GET("/api/createPlaylist/:id", handler)
	r.GET("/api/records/", crawler.PrintRecordsOfTheWeek)
	r.GET("/api/records/:id", crawler.GetRecord)
	r.POST("/playlistTimerTrigger", handler) // used by timer trigger, therefore no /api prefix
	r.Run(get_port())

}

func handler(c *gin.Context) {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	if c.Param("id") == "" {
		playlistID = spotify.ID(os.Getenv("PLAYLIST_ID"))

	} else {
		playlistID = spotify.ID(c.Param("id"))
	}

	log.Println("Plattentests.de Highlights of the week playlist generator")
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

	// login to spotify, all error messages are dealt within the function
	client := myauth.VerifyLogin()
	ctx := context.Background()

	log.Println("Emptying playlist...")
	client.ReplacePlaylistTracks(ctx, playlistID)

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
				notFound = append(notFound, record.Band+" - "+track)
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

	// out some json with all records that should have been added and the once that have not been added
	outputJson := make(map[string]interface{})
	outputJson["highlights"] = highlights
	outputJson["notFound"] = notFound

	c.IndentedJSON(http.StatusOK, outputJson)
}

// searches a song given by the track and record name and returns spotify.ID if successful
func searchSong(client spotify.Client, track string, record crawler.Record) spotify.ID {
	searchTerm := sanitizeTrackname(record.Band + " " + track)
	searchTerm = searchTerm + " " + record.Recordname

	// if record has a year, append it to the search
	if record.ReleaseYear != "" {
		searchTerm += " year:" + record.ReleaseYear
	}

	log.Printf(" searching term: %s", searchTerm)
	results, err := client.Search(context.Background(), searchTerm, spotify.SearchTypeTrack)
	if err != nil {
		log.Fatal(err)
	}
	// handle track results only if tracks are available
	if results.Tracks != nil && results.Tracks.Tracks != nil && len(results.Tracks.Tracks) > 0 {
		for i, item := range results.Tracks.Tracks {
			log.Printf(" found item: %s - %s  (%s)", item.Artists[0].Name, item.Name, item.Album.Name)
			// only get MAX_SEARCH_RESULTS results
			if i >= MAX_SEARCH_RESULTS-1 {
				break
			}
		}
		item := results.Tracks.Tracks[0]

		bandnameFromSearch := strings.ToLower(item.Artists[0].Name)
		bandnameFromPlattentests := strings.ToLower(record.Band)
		distance := levenshtein.DistanceForStrings([]rune(bandnameFromSearch), []rune(bandnameFromPlattentests), levenshtein.DefaultOptions)

		log.Println(" Levenshtein distance between", bandnameFromSearch, "and", bandnameFromPlattentests, ":", distance)
		threshold := 0.8

		calculatedThreshold := 1 - float64(distance)/float64(max(len(bandnameFromSearch), len(bandnameFromPlattentests)))
		if (calculatedThreshold) < threshold {
			log.Println(" Levenshtein distance too large")
			log.Printf(" not adding item %s - %s (%s) since artists don't match (%s != %s)", bandnameFromSearch, item.Name, item.Album.Name, bandnameFromPlattentests, bandnameFromSearch)
			return ""
		} else {

			// calculate the levenshtein distance between the trackname from the search and the trackname from the record
			tracknameFromSearch := strings.ToLower(item.Name)
			tracknameFromPlattentests := strings.ToLower(track)
			distance = levenshtein.DistanceForStrings([]rune(tracknameFromSearch), []rune(tracknameFromPlattentests), levenshtein.DefaultOptions)

			calculatedThreshold = 1 - float64(distance)/float64(max(len(tracknameFromSearch), len(tracknameFromPlattentests)))
			if (calculatedThreshold) < threshold {
				log.Println(" Levenshtein distance too large")
				log.Printf(" not adding item %s - %s (%s) since tracknames don't match (%s != %s)", bandnameFromSearch, item.Name, item.Album.Name, tracknameFromPlattentests, tracknameFromSearch)
				return ""
			}
		}
		log.Printf(" using item: %s - %s (%s)", bandnameFromSearch, item.Name, item.Album.Name)
		return item.ID
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
	_, err := client.AddTracksToPlaylist(context.Background(), playlistID, trackids...)
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

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
