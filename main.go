// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//       - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/plattentests-go/crawler"
	"github.com/zmb3/spotify"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:8080/callback"

// PlaylistID is
const PlaylistID = "2Gs8cKE9wSHo1t5iJEjTG1"

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopePlaylistModifyPrivate)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

func main() {
	// first start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go http.ListenAndServe(":8080", nil)

	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.ID)

	fmt.Println("Getting tracks of the week...")
	highlights := crawler.GetTracksOfTheWeek()
	fmt.Println("size of highlights in total: ", len(highlights))

	fmt.Println("Empying playlist...")
	client.ReplacePlaylistTracks(PlaylistID)

	fmt.Println("Adding highlights of the week to playlist....")
	for _, track := range highlights {
		fmt.Println(track)
		searchAndAddSong(client, track)
	}

}

func searchAndAddSong(client *spotify.Client, searchTerm string) {
	//results, err := client.Search(searchTerm, spotify.SearchTypeTrack|spotify.SearchTypePlaylist|spotify.SearchTypeAlbum)
	results, err := client.Search(searchTerm, spotify.SearchTypeTrack)
	if err != nil {
		log.Fatal(err)
	}
	// handle track results
	// if results.Tracks != nil {
	// 	fmt.Println("Tracks:")
	// 	for _, item := range results.Tracks.Tracks {
	// 		fmt.Println("   ", item.Name)
	// 		fmt.Println("add item to playlist...")
	// 		_, err := client.AddTracksToPlaylist(PlaylistID, item.ID)
	// 		if err != nil {
	// 			log.Fatalf("could not add track to playlist: %v", err)
	// 		}
	// 	}
	// }

	// handle track results
	if results.Tracks != nil && results.Tracks.Tracks != nil && len(results.Tracks.Tracks) > 0 {
		fmt.Println("Track:")
		item := results.Tracks.Tracks[0]
		fmt.Println("   ", item.Name)
		fmt.Println("add item to playlist...")
		_, err := client.AddTracksToPlaylist(PlaylistID, item.ID)
		if err != nil {
			log.Fatalf("could not add track to playlist: %v", err)
		}

	}
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
