package main

import (
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	creator "github.com/jetzlstorfer/plattentests-go/cmd/creator"
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
	Headline    string  `json:"Headline"`
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
	// Create a new Gin router
	r := gin.Default()
	r.Static("./assets", "./assets")
	r.StaticFile("./favicon.ico", "./assets/favicon.ico")
	r.StaticFile("/manifest.json", "./assets/manifest.json")

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
			recordOfTheWeek = strings.Trim(recordOfTheWeek, " ")

			// put record of the week on top of the playlist
			for i, record := range records {
				if record.Band == recordOfTheWeek {
					log.Println("record of the week found: " + recordOfTheWeek)
					records[i].IsRecordOfTheWeek = true
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

		data := commonTemplateData(c)
		data["Records"] = records

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}

	})

	r.GET("/search", func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("q"))

		var records []crawler.Record
		if query != "" {
			records = crawler.Search(query)
			// sort by score, descending — same default as the home page
			sort.Slice(records, func(i, j int) bool {
				return records[i].Score > records[j].Score
			})
		}

		tmpl, err := template.ParseFiles("templates/search.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Fatalf("Error parsing search templates: %v", err)
		}

		data := commonTemplateData(c)
		data["Query"] = query
		data["Records"] = records

		if err := tmpl.Execute(c.Writer, data); err != nil {
			log.Fatalf("Error executing search template: %v", err)
		}
	})

	r.GET("/playlist", func(c *gin.Context) {
		playlistID := os.Getenv("PLAYLIST_ID_PROD")

		tmpl, err := template.ParseFiles("templates/playlist.tmpl", "templates/utils.tmpl")
		if err != nil {
			log.Fatalf("Error parsing playlist templates: %v", err)
		}

		data := commonTemplateData(c)
		data["PlaylistID"] = playlistID

		if err := tmpl.Execute(c.Writer, data); err != nil {
			log.Fatalf("Error executing playlist template: %v", err)
		}
	})

	r.GET("/createPlaylist", func(c *gin.Context) {

		// Require authentication via Azure Container Apps Easy Auth.
		// The X-MS-CLIENT-PRINCIPAL-NAME header is injected by the platform after a successful
		// login; its absence means the request is unauthenticated.
		principal := easyAuthPrincipal(c)
		requireEasyAuth := easyAuthEnabled(c.Request)
		if requireEasyAuth && principal == "" {
			loginURL := easyAuthLoginURL(c.Request)
			log.Printf("unauthenticated request to /createPlaylist, redirecting to Easy Auth login: %s", loginURL)
			c.Redirect(http.StatusTemporaryRedirect, loginURL)
			return
		}
		if requireEasyAuth {
			log.Printf("user authenticated via Easy Auth: %s", principal)
		} else {
			log.Printf("Easy Auth disabled for this request, allowing local access to /createPlaylist")
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

			// put record of the week on top of the playlist
			for i, record := range highlights.Records {
				if record.Band == recordOfTheWeek {
					highlights.Records[i].IsRecordOfTheWeek = true
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

		data := commonTemplateData(c)
		data["Records"] = highlights

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

// easyAuthPrincipal returns the value of the X-MS-CLIENT-PRINCIPAL-NAME header injected by
// Azure Container Apps Easy Auth (https://learn.microsoft.com/azure/container-apps/authentication).
// The value is the authenticated user's display name or User Principal Name (UPN), depending on
// the identity provider configuration. Returns an empty string when the header is absent, meaning
// the request is unauthenticated or Easy Auth is not enabled.
func easyAuthPrincipal(c *gin.Context) string {
	return c.GetHeader("X-MS-CLIENT-PRINCIPAL-NAME")
}

func easyAuthLoginURL(r *http.Request) string {
	if !easyAuthEnabled(r) {
		return "/createPlaylist"
	}

	requestURI := "/createPlaylist"
	if r != nil && r.URL != nil {
		requestURI = r.URL.RequestURI()
	}

	return "/.auth/login/aad?post_login_redirect_uri=" + url.QueryEscape(requestURI)
}

// easyAuthEnabled controls whether Easy Auth should be enforced for this request.
// It can be explicitly set with EASY_AUTH_ENABLED=true|false. When unset, localhost
// requests default to disabled to support local development without ACA Easy Auth.
func easyAuthEnabled(r *http.Request) bool {
	if configured := strings.TrimSpace(os.Getenv("EASY_AUTH_ENABLED")); configured != "" {
		enabled, err := strconv.ParseBool(configured)
		if err == nil {
			return enabled
		}
		log.Printf("invalid EASY_AUTH_ENABLED value %q, falling back to host-based default", configured)
	}

	return !isLocalHostRequest(r)
}

func isLocalHostRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	host := strings.TrimSpace(r.Host)
	if host == "" {
		return false
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

func commonTemplateData(c *gin.Context) map[string]interface{} {
	principal := easyAuthPrincipal(c)
	authEnabled := easyAuthEnabled(c.Request)

	data := make(map[string]interface{})
	data["GitInfo"] = getCommitInfo()
	data["IsAuthenticated"] = !authEnabled || principal != ""
	data["LoginURL"] = easyAuthLoginURL(c.Request)

	return data
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
