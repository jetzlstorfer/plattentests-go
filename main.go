// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//       - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/jetzlstorfer/plattentests-go/crawler"
	"github.com/zmb3/spotify"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:8080/callback"

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopePlaylistModifyPrivate)
	ch    = make(chan *spotify.Client)
	state = "myCrazyState"
)
var (
	version string
	date    string
)

var playlistID spotify.ID
var plID = flag.String("playlistid", "", "The id of the playlist to be modified")

func main() {
	fmt.Println("Plattentests.de Highlights of the week playlist generator")
	fmt.Printf("version=%s, date=%s\n", version, date)
	fmt.Println()

	playlistID = spotify.ID(os.Getenv("PLAYLIST_ID"))

	flag.Parse()

	if *plID != "" {
		playlistID = spotify.ID(*plID)
	} else if playlistID == "" && *plID == "" {
		fmt.Fprintf(os.Stderr, "Error: missing playlist ID. Either use CLI flag or env variabble PLAYLIST_ID\n")
		flag.Usage()
		return
	}

	if playlistID == "" || os.Getenv("SPOTIFY_ID") == "" || os.Getenv("SPOTIFY_SECRET") == "" {
		log.Fatalln("PLAYLIST_ID, SPOTIFY_ID, or SPOTIFY_SECRET missing")
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
		if record.Name == recordOfTheWeek {
			highlights = append(highlights[:i], highlights[i+1:]...)
			highlights = append([]crawler.Record{record}, highlights...)
			break
		}
	}
	log.Println("Size of records of the week: ", len(highlights))

	// for _, record := range highlights {
	// 	log.Println(record.Name + ": " + strconv.Itoa(record.Score))
	// }

	// first start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go http.ListenAndServe(":8080", nil)

	url := auth.AuthURL(state)
	log.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("You are logged in as:", user.ID)
	log.Println("Empying playlist...")
	client.ReplacePlaylistTracks(playlistID)

	log.Println("Adding highlights of the week to playlist....")
	total := 0
	var notFound []string
	for _, record := range highlights {
		log.Println(record.Name + ": " + record.Link)
		for _, track := range record.Tracks {
			searchTerm := sanitizeTrackname(track)
			if !searchAndAddSong(client, searchTerm) {
				notFound = append(notFound, searchTerm+" /// "+track)
			}
			total++
		}
	}
	log.Println()
	log.Println("total tracks:     ", total)
	//log.Println("found tracks:     ", foundTracks)
	log.Println("not found tracks: ", len(notFound))
	log.Println("Not found search terms: ")
	for _, track := range notFound {
		log.Println(track)
	}

}

func searchAndAddSong(client *spotify.Client, searchTerm string) bool {
	//results, err := client.Search(searchTerm, spotify.SearchTypeTrack|spotify.SearchTypePlaylist|spotify.SearchTypeAlbum)
	results, err := client.Search(searchTerm, spotify.SearchTypeTrack)
	if err != nil {
		log.Fatal(err)
	}
	// handle track results
	if results.Tracks != nil && results.Tracks.Tracks != nil && len(results.Tracks.Tracks) > 0 {
		item := results.Tracks.Tracks[0]
		log.Println("", item.Name)
		//log.Println("add item to playlist...")
		if item.ID != "" {
			_, err := client.AddTracksToPlaylist(playlistID, item.ID)
			if err != nil {
				log.Fatalf("could not add track %v to playlist: %v", item.Name, err)
			}
			return true
		}
		return false
	}
	return false

}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	fmt.Fprintf(w, "Login Completed!")
	ch <- &client
}

func sanitizeTrackname(trackname string) string {
	sanitizedName := trackname
	sanitizedName = strings.Split(sanitizedName, "(feat. ")[0]
	sanitizedName = strings.Split(sanitizedName, "(with ")[0]
	sanitizedName = strings.Split(sanitizedName, "(Bonus)")[0]
	return sanitizedName
}
