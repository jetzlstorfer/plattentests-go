package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	"golang.org/x/text/encoding/charmap"
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
	r.StaticFile("./favicon.ico", "./assets/favicon.ico")

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

		// sort by score
		if c.DefaultQuery("sort", "title") == "score" {
			sort.Slice(records, func(i, j int) bool {
				return records[i].Score > records[j].Score
			})

			// put record of the week on top of the playlist
			recordOfTheWeek := crawler.GetRecordOfTheWeekBandName()
			recordOfTheWeek, _ = charmap.ISO8859_1.NewDecoder().String(recordOfTheWeek)

			// put record of the week on top of the playlist
			for i, record := range records {
				if record.Band == recordOfTheWeek {
					records[0], records[i] = records[i], records[0]
					break
				}
			}
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

		// Check if the user is authenticated
		user, password, ok := c.Request.BasicAuth()
		if !ok || !checkAuth(user, password) {
			c.Header("WWW-Authenticate", "Basic realm=\"Restricted Content\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		playlist := c.DefaultQuery("playlist", "")
		playlistID := os.Getenv("PLAYLIST_ID")
		if playlist == "prod" {
			playlistID = os.Getenv("PLAYLIST_ID_PROD")
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
	if username != os.Getenv(("CREATOR_USER")) || password != os.Getenv("CREATOR_PASSWORD") {
		return false
	}
	return true
}
