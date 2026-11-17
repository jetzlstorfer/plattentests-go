package main

import (
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	creator "github.com/jetzlstorfer/plattentests-go/cmd/creator"
	"github.com/jetzlstorfer/plattentests-go/internal/observability"
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
	ReleaseDate string  `json:"ReleaseDate"`
	ReleaseYear string  `json:"ReleaseYear"`
	Tracks      []Track `json:"Tracks"`
	Headline    string  `json:"Headline"`
	Description string  `json:"Description"`
}

type Track struct {
	Band        string
	Trackname   string
	Tracklink   string
	IsHighlight bool
}

type Highlights struct {
	Records    []Record `json:"Highlights"`
	NotFound   []string `json:"NotFound"`
	PlaylistID string   `json:"PlaylistID"`
}

func main() {
	observability.ConfigureLogging("plattentests-web")

	r := gin.New()
	r.Use(requestLogger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.ErrorContext(c.Request.Context(), "request panic recovered",
			"error", recovered,
			"stack", string(debug.Stack()),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	r.Static("./assets", "./assets")
	r.StaticFile("./favicon.ico", "./assets/favicon.ico")
	r.StaticFile("/manifest.json", "./assets/manifest.json")

	// Define a handler function for the root endpoint
	r.GET("/", func(c *gin.Context) {

		records, err := crawler.GetRecordsOfTheWeekSafe()
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to load records of the week", "error", err)
			tmpl, tmplErr := template.ParseFiles("templates/utils.tmpl")
			if tmplErr != nil {
				c.String(http.StatusInternalServerError, "Could not load records of the week")
				return
			}
			c.Status(http.StatusInternalServerError)
			_ = tmpl.ExecuteTemplate(c.Writer, "ErrorPage", commonTemplateData(c))
			return
		}

		// sort by score
		if c.DefaultQuery("sort", "score") == "score" {
			sort.Slice(records, func(i, j int) bool {
				return records[i].Score > records[j].Score
			})

			// put record of the week on top of the playlist
			recordOfTheWeek, err := crawler.GetRecordOfTheWeekBandNameSafe()
			if err != nil {
				slog.WarnContext(c.Request.Context(), "could not load record of the week", "error", err)
				recordOfTheWeek = ""
			}
			recordOfTheWeek = strings.Trim(recordOfTheWeek, " ")

			// put record of the week on top of the playlist
			for i, record := range records {
				if record.Band == recordOfTheWeek {
					slog.DebugContext(c.Request.Context(), "record of the week found", "artist", recordOfTheWeek)
					records[i].IsRecordOfTheWeek = true
					records[0], records[i] = records[i], records[0]
					break
				}
				slog.DebugContext(c.Request.Context(), "record is not the record of the week",
					"artist", record.Band,
					"record_of_the_week", recordOfTheWeek,
				)
			}
		}

		// Load the template file
		tmpl, err := template.ParseFiles("templates/records.tmpl", "templates/utils.tmpl")
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to parse records template", "error", err)
			c.String(http.StatusInternalServerError, "Could not load records")
			return
		}

		data := commonTemplateData(c)
		data["Records"] = records

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to render records template", "error", err)
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
			slog.ErrorContext(c.Request.Context(), "failed to parse search templates", "error", err)
			c.String(http.StatusInternalServerError, "Could not load search")
			return
		}

		data := commonTemplateData(c)
		data["Query"] = query
		data["Records"] = records

		if err := tmpl.Execute(c.Writer, data); err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to render search template", "error", err)
		}
	})

	r.GET("/playlist", func(c *gin.Context) {
		playlistID := os.Getenv("PLAYLIST_ID_PROD")

		tmpl, err := template.ParseFiles("templates/playlist.tmpl", "templates/utils.tmpl")
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to parse playlist templates", "error", err)
			c.String(http.StatusInternalServerError, "Could not load playlist")
			return
		}

		data := commonTemplateData(c)
		data["PlaylistID"] = playlistID

		if err := tmpl.Execute(c.Writer, data); err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to render playlist template", "error", err)
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
			slog.InfoContext(c.Request.Context(), "unauthenticated request; redirecting to Easy Auth login")
			c.Redirect(http.StatusTemporaryRedirect, loginURL)
			return
		}
		if requireEasyAuth {
			slog.InfoContext(c.Request.Context(), "user authenticated via Easy Auth")
		} else {
			slog.DebugContext(c.Request.Context(), "Easy Auth disabled; allowing local playlist creation")
		}

		playlist := c.DefaultQuery("playlist", "")
		playlistID := os.Getenv("PLAYLIST_ID")
		if playlist == "prod" {
			playlistID = os.Getenv("PLAYLIST_ID_PROD")
		}

		tmpl, err := template.ParseFiles("templates/createPlaylist.tmpl", "templates/utils.tmpl")
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to parse create-playlist template", "error", err)
			c.String(http.StatusInternalServerError, "Could not load playlist creation page")
			return
		}

		data := commonTemplateData(c)
		data["Records"] = creator.Result{PlaylistID: playlistID}

		results, err := creator.CreatePlaylist(playlistID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to create playlist", "error", err)
			data["CreatePlaylistError"] = friendlyCreatePlaylistError(err)
			c.Status(http.StatusServiceUnavailable)
			if execErr := tmpl.Execute(c.Writer, data); execErr != nil {
				slog.ErrorContext(c.Request.Context(), "failed to render create-playlist error", "error", execErr)
			}
			return
		}
		highlights := results
		if highlights.PlaylistID == "" {
			highlights.PlaylistID = playlistID
		}

		// sort by score
		if c.DefaultQuery("sort", "score") == "score" {
			sort.Slice(highlights.Records, func(i, j int) bool {
				return highlights.Records[i].Score > highlights.Records[j].Score
			})

			// put record of the week on top of the playlist
			recordOfTheWeek, err := crawler.GetRecordOfTheWeekBandNameSafe()
			if err != nil {
				slog.WarnContext(c.Request.Context(), "could not load record of the week", "error", err)
				recordOfTheWeek = ""
			}

			// put record of the week on top of the playlist
			for i, record := range highlights.Records {
				if record.Band == recordOfTheWeek {
					highlights.Records[i].IsRecordOfTheWeek = true
					highlights.Records[0], highlights.Records[i] = highlights.Records[i], highlights.Records[0]
					break
				}
			}
		}

		data["Records"] = highlights

		// Execute the template with the record data
		if err := tmpl.Execute(c.Writer, data); err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to render create-playlist template", "error", err)
		}
	})

	// Start the server
	if err := r.Run(":8081"); err != nil {
		slog.Error("failed to start web server", "error", err)
		os.Exit(1)
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		slog.InfoContext(c.Request.Context(), "http request",
			"http.request.method", c.Request.Method,
			"http.route", route,
			"http.response.status_code", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
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
		slog.Warn("invalid EASY_AUTH_ENABLED value; using host-based default", "value", configured)
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
	slog.Debug("reading commit info", "git_sha", os.Getenv("GIT_SHA"))
	if os.Getenv("GIT_SHA") != "" {
		return os.Getenv("GIT_SHA")
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}

func friendlyCreatePlaylistError(err error) string {
	if err == nil {
		return "Unknown error"
	}

	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "temporarily_unavailable") {
		return "Spotify authentication is temporarily unavailable. Please try again in a minute. Technical details: " + msg
	}

	return msg
}
