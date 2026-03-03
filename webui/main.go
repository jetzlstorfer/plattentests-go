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
	"github.com/gorilla/sessions"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	creator "github.com/jetzlstorfer/plattentests-go/cmd/creator"
	"golang.org/x/text/encoding/charmap"
)

const sessionName = "plattentests-session"

var store *sessions.CookieStore

func initStore() {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		log.Fatal("SESSION_SECRET environment variable is not set")
	}
	store = sessions.NewCookieStore([]byte(secret))
	secure := os.Getenv("SESSION_SECURE") == "true"
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

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

func isAuthenticated(c *gin.Context) bool {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return false
	}
	auth, ok := session.Values["authenticated"].(bool)
	return ok && auth
}

func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAuthenticated(c) {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func main() {
	initStore()

	// Create a new Gin router
	r := gin.Default()
	r.Static("./assets", "./assets")
	r.StaticFile("./favicon.ico", "./assets/favicon.ico")

	r.GET("/login", func(c *gin.Context) {
		if isAuthenticated(c) {
			c.Redirect(http.StatusFound, "/")
			return
		}
		tmpl, err := template.ParseFiles("templates/login.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Printf("Error parsing login template: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		data := make(map[string]interface{})
		data["GitInfo"] = getCommitInfo()
		if err := tmpl.Execute(c.Writer, data); err != nil {
			log.Printf("Error executing login template: %v", err)
		}
	})

	r.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		if !checkAuth(username, password) {
			tmpl, err := template.ParseFiles("templates/login.tmpl", "templates/utils.tmpl")
			if err != nil {
				log.Printf("Error parsing login template: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			data := make(map[string]interface{})
			data["GitInfo"] = getCommitInfo()
			data["Error"] = "Invalid username or password"
			c.Writer.WriteHeader(http.StatusUnauthorized)
			if err := tmpl.Execute(c.Writer, data); err != nil {
				log.Printf("Error executing login template: %v", err)
			}
			return
		}

		session, err := store.Get(c.Request, sessionName)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		session.Values["authenticated"] = true
		if err := session.Save(c.Request, c.Writer); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Redirect(http.StatusFound, "/")
	})

	r.GET("/logout", func(c *gin.Context) {
		session, err := store.Get(c.Request, sessionName)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}
		session.Values["authenticated"] = false
		session.Options.MaxAge = -1
		if err := session.Save(c.Request, c.Writer); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Redirect(http.StatusFound, "/login")
	})

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
		data["IsLoggedIn"] = isAuthenticated(c)

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}

	})

	r.GET("/createPlaylist", requireAuth(), func(c *gin.Context) {

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
		data["IsLoggedIn"] = true

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
