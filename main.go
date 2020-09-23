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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/oauth2"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
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
		TargetPlaylist string `envconfig:"PLAYLIST_ID" required:"true"`
		ProdPlaylist   string `envconfig:"PROD_PLAYLIST_ID"`
		Bucket         string `required:"true"`
		TokenFile      string `envconfig:"TOKEN_FILE" required:"true"`
		Region         string `required:"true"`
		LogFile        string `required:"false"`
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
	if os.Getenv("LOCAL_EXECUTION") == "true" {
		log.Println("executing locally")
		handler(events.APIGatewayProxyRequest{})
	} else {
		log.Println("executing as lambda function")
		lambda.Start(handler)
	}
}

func handler(request events.APIGatewayProxyRequest) {
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

	flag.Parse()

	if request.Path == "prod" || *prod == "true" {
		log.Println("Generating production playlist from testing playlist")
		client, _ := verifyLogin()
		log.Println("Getting tracks from testing playlist")
		tracks, err := client.GetPlaylistTracks(playlistID)
		if err != nil {
			log.Println("Error fetching tracks from playlist")
		}
		log.Printf("Fetched %d tracks from testing playlist", len(tracks.Tracks))
		var trackIDs []spotify.ID
		for _, track := range tracks.Tracks {
			trackIDs = append(trackIDs, track.Track.ID)
		}
		err = client.ReplacePlaylistTracks(spotify.ID(config.ProdPlaylist), trackIDs...)
		if err != nil {
			log.Printf("Could not populate production playlist: %s", err.Error())
		}
		log.Println("Copying from testing to production done.")
		return
	}

	log.Println("Plattentests.de Highlights of the week playlist generator")
	log.Printf("version=%s, date=%s\n", version, date)
	log.Println()

	if *plID != "" {
		playlistID = spotify.ID(*plID)
	} else if playlistID == "" && *plID == "" {
		fmt.Fprintf(os.Stderr, "Error: missing playlist ID. Either use CLI flag or env variabble PLAYLIST_ID\n")
		flag.Usage()
		return
	}

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

	// local file option
	// buff, err := ioutil.ReadFile("mytoken.txt")
	// if err != nil {
	// 	log.Fatalf("could not read token file: %v", err)
	// }

	log.Println("---")
	log.Println("Connecting to Spotify")
	log.Println("---")
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

func verifyLogin() (spotify.Client, error) {
	log.Println("Connecting to AWS to download token")
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(config.Region)}))
	s3dl := s3manager.NewDownloader(sess)
	s3ul := s3manager.NewUploader(sess)

	// Download the token file from S3.
	buff := &aws.WriteAtBuffer{}
	if _, err := s3dl.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(config.Bucket),
		Key:    aws.String(config.TokenFile),
	}); err != nil {
		log.Fatalf("failed to download token file from S3: %v", err)
	}
	log.Println("Token downloaded from S3: ", config.TokenFile)

	tok := new(oauth2.Token)
	if err := json.Unmarshal(buff.Bytes(), tok); err != nil {
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

	btys, err := json.Marshal(newToken)
	if err != nil {
		log.Fatalf("could not marshal token: %v", err)
	}

	if _, err := s3ul.Upload(&s3manager.UploadInput{
		Bucket: aws.String(config.Bucket),
		Key:    aws.String(config.TokenFile),
		Body:   bytes.NewReader(btys),
	}); err != nil {
		log.Fatalf("could not write token to s3: %v", err)
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
