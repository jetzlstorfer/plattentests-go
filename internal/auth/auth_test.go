package auth

import (
	"errors"
	"testing"
	"time"
)

func TestIsRetryableOAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error is not retryable", nil, false},
		{"temporarily_unavailable is retryable", errors.New("oauth2: \"temporarily_unavailable\""), true},
		{"timeout is retryable", errors.New("net/http: request canceled (Client.Timeout exceeded)"), true},
		{"connection reset is retryable", errors.New("read tcp: connection reset by peer"), true},
		{"server error is retryable", errors.New("oauth2: server error"), true},
		{"uppercase Timeout is retryable (case-insensitive)", errors.New("Timeout while contacting Spotify"), true},
		{"invalid_grant is not retryable", errors.New("oauth2: \"invalid_grant\""), false},
		{"unauthorized is not retryable", errors.New("401 unauthorized"), false},
		{"empty message is not retryable", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableOAuthError(tt.err); got != tt.want {
				t.Errorf("isRetryableOAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableAzureBlobError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error is not retryable", nil, false},
		{"dns no such host is retryable", errors.New("dial tcp: lookup storage.blob.core.windows.net: no such host"), true},
		{"temporary dns failure is retryable", errors.New("lookup storage: temporary failure in name resolution"), true},
		{"timeout is retryable", errors.New("context deadline exceeded (Client.Timeout exceeded while awaiting headers)"), true},
		{"connection reset is retryable", errors.New("read tcp: connection reset by peer"), true},
		{"auth error is not retryable", errors.New("authentication failed"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableAzureBlobError(tt.err); got != tt.want {
				t.Errorf("isRetryableAzureBlobError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		name      string
		attempt   int
		baseDelay time.Duration
		want      time.Duration
	}{
		{"first attempt uses base delay", 1, 2 * time.Second, 2 * time.Second},
		{"second attempt doubles delay", 2, 2 * time.Second, 4 * time.Second},
		{"third attempt doubles again", 3, 2 * time.Second, 8 * time.Second},
		{"delay capped at 30 seconds", 5, 2 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryDelay(tt.attempt, tt.baseDelay); got != tt.want {
				t.Errorf("retryDelay(%d, %s) = %s, want %s", tt.attempt, tt.baseDelay, got, tt.want)
			}
		})
	}
}
