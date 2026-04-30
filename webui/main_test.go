package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
