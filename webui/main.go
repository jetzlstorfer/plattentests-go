package main

import (
	"html/template"
	"net/http"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	creator "github.com/jetzlstorfer/plattentests-go/cmd/creator"
	"github.com/jetzlstorfer/plattentests-go/internal/logging"
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
	Description string  `json:"Description"`
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
	// Initialize OpenTelemetry logging
	if err := logging.Init(); err != nil {
		logging.Fatal("Failed to initialize logging: %v", err)
	}
	defer logging.Shutdown()

	// Initialize OpenTelemetry tracing
	if err := logging.InitTracing("plattentests-webui"); err != nil {
		logging.Fatal("Failed to initialize tracing: %v", err)
	}
	defer logging.ShutdownTracing()

	// Create a new Gin router
	r := gin.Default()
	r.Static("./assets", "./assets")
	r.StaticFile("./favicon.ico", "./assets/favicon.ico")

	// Define a handler function for the root endpoint
	r.GET("/", func(c *gin.Context) {
		ctx, span := logging.StartSpan(c.Request.Context(), "GET /")
		defer span.End()

		logging.InfoWithSpan(ctx, "Fetching records of the week")
		records := crawler.GetRecordsOfTheWeek()
		logging.AddSpanEvent(ctx, "Records fetched", logging.Attribute("count", len(records)))

		// sort by score
		if c.DefaultQuery("sort", "score") == "score" {
			ctx, sortSpan := logging.StartSpan(ctx, "sort-records")
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
					logging.Info("record of the week found: %s", recordOfTheWeek)
					logging.AddSpanEvent(ctx, "Record of the week found", logging.Attribute("band", recordOfTheWeek))
					records[0], records[i] = records[i], records[0]
					break
				}
				logging.Debug("record of the week not found: %s vs %s", record.Band, recordOfTheWeek)
			}
			sortSpan.End()
		}

		// Load the template file
		ctx, templateSpan := logging.StartSpan(ctx, "render-template")
		tmpl, err := template.ParseFiles("templates/records.tmpl", "templates/utils.tmpl")
		if err != nil {
			logging.ErrorWithSpan(ctx, err, "Error parsing template: %v", err)
			logging.Fatal("Error parsing template: %v", err)
		}

		data := make(map[string]interface{})
		data["Records"] = records
		data["GitInfo"] = getCommitInfo()

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			logging.ErrorWithSpan(ctx, err, "Error executing template: %v", err)
			logging.Fatal("Error executing template: %v", err)
		}
		templateSpan.End()

	})

	r.GET("/createPlaylist", func(c *gin.Context) {
		ctx, span := logging.StartSpan(c.Request.Context(), "GET /createPlaylist")
		defer span.End()

		// Check if the user is authenticated
		user, password, ok := c.Request.BasicAuth()
		if !ok || !checkAuth(user, password) {
			c.Header("WWW-Authenticate", "Basic realm=\"Restricted Content\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			logging.Warn("could not authenticate user")
			logging.AddSpanEvent(ctx, "Authentication failed")
			return
		}
		logging.AddSpanEvent(ctx, "User authenticated", logging.Attribute("user", user))

		playlist := c.DefaultQuery("playlist", "")
		playlistID := os.Getenv("PLAYLIST_ID")
		if playlist == "prod" {
			playlistID = os.Getenv("PLAYLIST_ID_PROD")
		}
		logging.AddSpanAttributes(ctx, logging.Attribute("playlist_id", playlistID), logging.Attribute("playlist_type", playlist))

		ctx, createSpan := logging.StartSpan(ctx, "create-playlist")
		logging.InfoWithSpan(ctx, "Creating playlist")
		results := creator.CreatePlaylist(playlistID)
		var highlights creator.Result
		highlights.Records = results.Records
		highlights.NotFound = results.NotFound
		highlights.PlaylistID = playlistID
		logging.AddSpanEvent(ctx, "Playlist created",
			logging.Attribute("tracks_found", len(results.Records)),
			logging.Attribute("tracks_not_found", len(results.NotFound)))
		createSpan.End()

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
		ctx, templateSpan := logging.StartSpan(ctx, "render-playlist-template")
		tmpl, err := template.ParseFiles("templates/createPlaylist.tmpl", "templates/utils.tmpl")
		if err != nil {
			logging.ErrorWithSpan(ctx, err, "Error parsing template: %v", err)
			logging.Fatal("Error parsing template: %v", err)
		}

		data := make(map[string]interface{})
		data["Records"] = highlights
		data["GitInfo"] = getCommitInfo()

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			logging.ErrorWithSpan(ctx, err, "Error executing template: %v", err)
			logging.Fatal("Error executing template: %v", err)
		}
		templateSpan.End()
	})

	// Start the server
	if err := r.Run(":8081"); err != nil {
		logging.Fatal("Error starting server: %v", err)
	}
}

func checkAuth(username, password string) bool {
	if username != os.Getenv(("CREATOR_USER")) || password != os.Getenv("CREATOR_PASSWORD") {
		logging.Warn("wrong credentials")
		return false
	}
	return true
}

func getCommitInfo() string {
	sha := os.Getenv("GIT_SHA")
	logging.Debug("get commit info: %s", sha)
	if sha != "" {
		return sha
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
