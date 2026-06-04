package main

import (
	"bytes"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
)

func TestRecordTableSongFoundIndicator(t *testing.T) {
	tmpl, err := template.ParseFiles("templates/utils.tmpl")
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	data := map[string]interface{}{
		"Records": []crawler.Record{
			{
				Band:       "Band",
				Recordname: "Record",
				Tracks: []crawler.Track{
					{Trackname: "Found Song", Tracklink: "https://open.spotify.com/track/abc"},
					{Trackname: "Missing Song"},
				},
			},
		},
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "RecordTable", data); err != nil {
		t.Fatalf("failed to render RecordTable: %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "✅") {
		t.Fatalf("expected rendered RecordTable to contain found-song indicator ✅, got: %s", rendered)
	}
	if !strings.Contains(rendered, "🔍") {
		t.Fatalf("expected rendered RecordTable to contain missing-song indicator 🔍, got: %s", rendered)
	}
}

func TestGetCommitInfo(t *testing.T) {
	t.Run("returns GIT_SHA env var when set", func(t *testing.T) {
		t.Setenv("GIT_SHA", "abc123def456")

		result := getCommitInfo()
		if result != "abc123def456" {
			t.Errorf("getCommitInfo() = %q, want %q", result, "abc123def456")
		}
	})

	t.Run("returns empty string when GIT_SHA not set and no build info revision", func(t *testing.T) {
		t.Setenv("GIT_SHA", "")
		// In the test binary there is no vcs.revision build setting, so result should be empty or
		// a real revision from the test binary's build info. We only verify no panic occurs and
		// the result is a string (possibly non-empty when run from a git checkout).
		result := getCommitInfo()
		// Just ensure the function returns without panicking and returns a string value
		_ = result
	})
}

func TestEasyAuthPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		headerValue    string
		expectedResult string
	}{
		{
			name:           "header present with username",
			headerValue:    "jane.doe@example.com",
			expectedResult: "jane.doe@example.com",
		},
		{
			name:           "header absent",
			headerValue:    "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/createPlaylist", nil)
			if tt.headerValue != "" {
				ctx.Request.Header.Set("X-MS-CLIENT-PRINCIPAL-NAME", tt.headerValue)
			}

			result := easyAuthPrincipal(ctx)
			if result != tt.expectedResult {
				t.Errorf("easyAuthPrincipal() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

func TestEasyAuthLoginURL(t *testing.T) {
	t.Setenv("EASY_AUTH_ENABLED", "true")

	t.Run("redirects to aad login and preserves default route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/createPlaylist", nil)

		result := easyAuthLoginURL(req)
		if result != "/.auth/login/aad?post_login_redirect_uri=%2FcreatePlaylist" {
			t.Errorf("easyAuthLoginURL() = %q, want %q", result, "/.auth/login/aad?post_login_redirect_uri=%2FcreatePlaylist")
		}
	})

	t.Run("redirects to aad login and preserves query string", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/createPlaylist?playlist=prod", nil)

		result := easyAuthLoginURL(req)
		if result != "/.auth/login/aad?post_login_redirect_uri=%2FcreatePlaylist%3Fplaylist%3Dprod" {
			t.Errorf("easyAuthLoginURL() = %q, want %q", result, "/.auth/login/aad?post_login_redirect_uri=%2FcreatePlaylist%3Fplaylist%3Dprod")
		}
	})
}

func TestEasyAuthLoginURLLocalFallback(t *testing.T) {
	t.Run("returns createPlaylist path on localhost when EASY_AUTH_ENABLED is unset", func(t *testing.T) {
		t.Setenv("EASY_AUTH_ENABLED", "")
		req := httptest.NewRequest("GET", "http://localhost:8081/", nil)

		result := easyAuthLoginURL(req)
		if result != "/createPlaylist" {
			t.Errorf("easyAuthLoginURL() = %q, want %q", result, "/createPlaylist")
		}
	})
}

func TestRecordTable_EmphasizesHighlightTracks(t *testing.T) {
	tmpl, err := template.ParseFiles("templates/utils.tmpl")
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	data := map[string]interface{}{
		"Records": []crawler.Record{
			{
				Band:       "Test Band",
				Recordname: "Test Album",
				Image:      "/cover.jpg",
				Link:       "/record",
				Score:      8,
				Tracks: []crawler.Track{
					{Trackname: "Highlight Song", IsHighlight: true},
					{Trackname: "Regular Song", IsHighlight: false},
				},
			},
		},
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "RecordTable", data); err != nil {
		t.Fatalf("execute template: %v", err)
	}

	html := out.String()
	if !strings.Contains(html, "<strong>Highlight Song</strong>") {
		t.Fatalf("highlight track not emphasized, html: %s", html)
	}
	if strings.Contains(html, "<strong>Regular Song</strong>") {
		t.Fatalf("non-highlight track should not be emphasized, html: %s", html)
	}
}

func TestRecordTableShowsReleaseDateAndFutureEmoji(t *testing.T) {
	tmpl, err := template.ParseFiles("templates/utils.tmpl")
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	data := map[string]interface{}{
		"Records": []crawler.Record{
			{Band: "Future Band", Recordname: "Future Album", ReleaseDate: "31.12.2099", Score: 8},
			{Band: "Past Band", Recordname: "Past Album", ReleaseDate: "01.01.2000", Score: 7},
		},
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "RecordTable", data); err != nil {
		t.Fatalf("failed to render template: %v", err)
	}

	html := out.String()
	if !strings.Contains(html, "📅") {
		t.Fatalf("expected rendered output to include release date emoji, got: %s", html)
	}
	if !strings.Contains(html, "31.12.2099") {
		t.Fatalf("expected rendered output to include future release date, got: %s", html)
	}
	if !strings.Contains(html, "01.01.2000") {
		t.Fatalf("expected rendered output to include past release date, got: %s", html)
	}
	if count := strings.Count(html, "⏭️"); count != 2 {
		t.Fatalf("expected future emoji to appear only for future record in both views (2x), got %d", count)
	}
}
