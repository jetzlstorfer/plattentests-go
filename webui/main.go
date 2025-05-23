package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	creator "github.com/jetzlstorfer/plattentests-go/cmd/creator"
	"golang.org/x/text/encoding/charmap"
)

//const RecordEndPoint = "https://plattentests-go.azurewebsites.net/api/records/"
//const CreatePlaylistEndpoint = "https://plattentests-go.azurewebsites.net/api/createPlaylist/"

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

		records := crawler.GetRecordsOfTheWeek()

		// sort by score
		if c.DefaultQuery("sort", "score") == "score" {
			sort.Slice(records, func(i, j int) bool {
				return records[i].Score > records[j].Score
			})

			// put record of the week on top of the playlist
			recordOfTheWeek := crawler.GetRecordOfTheWeekBandName()
			recordOfTheWeek, _ = charmap.ISO8859_1.NewDecoder().String(recordOfTheWeek)
			recordOfTheWeek = strings.Trim(recordOfTheWeek, " ")

			// put record of the week on top of the playlist
			for i, record := range records {
				if record.Band == recordOfTheWeek {
					log.Println("record of the week found: " + recordOfTheWeek)
					records[0], records[i] = records[i], records[0]
					break
				}
				log.Println("record of the week not found: " + record.Band + " vs " + recordOfTheWeek)
			}
		}

		// Load the template file
		tmpl, err := template.ParseFiles("templates/records.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Fatalf("Error parsing template: %v", err)
		}

		data := make(map[string]interface{})
		data["Records"] = records
		data["GitInfo"] = getCommitInfo()

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}

	})

	r.GET("/createPlaylist", func(c *gin.Context) {

		// Check if the user is authenticated
		user, password, ok := c.Request.BasicAuth()
		if !ok || !checkAuth(user, password) {
			c.Header("WWW-Authenticate", "Basic realm=\"Restricted Content\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			log.Println("could not authenticate user")
			return
		}

		playlist := c.DefaultQuery("playlist", "")
		playlistID := os.Getenv("PLAYLIST_ID")
		if playlist == "prod" {
			playlistID = os.Getenv("PLAYLIST_ID_PROD")
		}

		results := creator.CreatePlaylist(playlistID)
		var highlights creator.Result
		highlights.Records = results.Records
		highlights.NotFound = results.NotFound
		highlights.PlaylistID = playlistID

		// sort by score
		if c.DefaultQuery("sort", "score") == "score" {
			sort.Slice(highlights.Records, func(i, j int) bool {
				return highlights.Records[i].Score > highlights.Records[j].Score
			})

			// put record of the week on top of the playlist
			recordOfTheWeek := crawler.GetRecordOfTheWeekBandName()
			recordOfTheWeek, _ = charmap.ISO8859_1.NewDecoder().String(recordOfTheWeek)

			// put record of the week on top of the playlist
			for i, record := range highlights.Records {
				if record.Band == recordOfTheWeek {
					highlights.Records[0], highlights.Records[i] = highlights.Records[i], highlights.Records[0]
					break
				}
			}
		}

		// Load the template file
		tmpl, err := template.ParseFiles("templates/createPlaylist.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Fatalf("Error parsing template: %v", err)
		}

		data := make(map[string]interface{})
		data["Records"] = highlights
		data["GitInfo"] = getCommitInfo()

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
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
		log.Println("wrong credentials")
		return false
	}
	return true
}

func getCommitInfo() string {
	log.Println("get commit info: " + os.Getenv("GIT_SHA"))
	if os.Getenv("GIT_SHA") != "" {
		return os.Getenv("GIT_SHA")
	} else {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}
	}
	return ""
}
