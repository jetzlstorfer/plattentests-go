package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const RecordEndPoint = "https://plattentests-go.azurewebsites.net/api/records/"
const CreatePlaylistEndpoint = "https://plattentests-go.azurewebsites.net/api/createPlaylist/"

// const RecordEndPoint = "http://localhost:8080/api/records/"
// const CreatePlaylistEndpoint = "http://localhost:8080/api/createPlaylist/"

// Record represents an record's information
type Record struct {
	Image       string  `json:"Image"`
	Band        string  `json:"Band"`
	Recordname  string  `json:"Recordname"`
	Link        string  `json:"Link"`
	Score       int     `json:"Score"`
	ReleaseYear string  `json:"ReleaseYear"`
	Tracks      []Track `json:"Tracks"`
}

type Track struct {
	Band      string
	Trackname string
	Tracklink string
}

type Highlights struct {
	Records    []Record `json:"Highlights"`
	NotFound   []string `json:"NotFound"`
	PlaylistID string   `json:"PlaylistID"`
}

func main() {
	// Create a new Gin router
	r := gin.Default()
	r.Static("./assets", "./assets")

	// Define a handler function for the root endpoint
	r.GET("/", func(c *gin.Context) {
		// Fetch the record data from the given URL
		resp, err := http.Get(RecordEndPoint)
		if err != nil {
			log.Fatalf("Error fetching record data: %v", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		// Unmarshal the JSON data into an array of Record objects
		var records []Record
		if err := json.Unmarshal(body, &records); err != nil {
			log.Fatalf("Error unmarshaling record data: %v", err)
		}

		// Load the template file
		tmpl, err := template.ParseFiles("templates/records.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Fatalf("Error parsing template: %v", err)
		}

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, records); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}
	})

	r.GET("/createPlaylist", func(c *gin.Context) {
		playlist := c.DefaultQuery("playlist", "")
		playlistID := os.Getenv("PLAYLIST_ID")
		if playlist == "prod" {
			playlistID = os.Getenv("PLAYLIST_ID_PROD")
			// Check if the user is authenticated
			user, password, ok := c.Request.BasicAuth()
			if !ok || !checkAuth(user, password) {
				c.Header("WWW-Authenticate", "Basic realm=\"Restricted Content\"")
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

		}

		myPlaylistEndpoint := CreatePlaylistEndpoint + playlistID
		// Fetch the record data from the given URL
		resp, err := http.Get(myPlaylistEndpoint)
		if err != nil {
			log.Fatalf("Error fetching record data: %v", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		// Unmarshal the JSON data into an array of record objects
		var highlights Highlights
		if err := json.Unmarshal(body, &highlights); err != nil {
			log.Fatalf("Error unmarshaling record data: %v", err)
		}
		highlights.PlaylistID = playlistID

		// Load the template file
		tmpl, err := template.ParseFiles("templates/createPlaylist.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Fatalf("Error parsing template: %v", err)
		}

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, highlights); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}
	})

	// Start the server
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func checkAuth(username, password string) bool {
	// TODO: Implement a proper authentication
	if username != "TODO" || password != "TODO" {
		return false
	}
	return true
}
