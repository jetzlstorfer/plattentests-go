package creator

import (
	"os"
	"testing"

	"github.com/zmb3/spotify/v2"
)
func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []spotify.ID
		expected []spotify.ID
	}{
		{
			name:     "empty list",
			input:    []spotify.ID{},
			expected: []spotify.ID{},
		},
		{
			name:     "no duplicates",
			input:    []spotify.ID{"a", "b", "c"},
			expected: []spotify.ID{"a", "b", "c"},
		},
		{
			name:     "all duplicates",
			input:    []spotify.ID{"x", "x", "x"},
			expected: []spotify.ID{"x"},
		},
		{
			name:     "duplicates at end",
			input:    []spotify.ID{"a", "b", "a"},
			expected: []spotify.ID{"a", "b"},
		},
		{
			name:     "duplicates in middle",
			input:    []spotify.ID{"a", "b", "b", "c"},
			expected: []spotify.ID{"a", "b", "c"},
		},
		{
			name:     "preserves insertion order",
			input:    []spotify.ID{"c", "b", "a", "b", "c"},
			expected: []spotify.ID{"c", "b", "a"},
		},
		{
			name:     "single element",
			input:    []spotify.ID{"only"},
			expected: []spotify.ID{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDuplicates(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("removeDuplicates(%v) length = %d, want %d", tt.input, len(result), len(tt.expected))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("removeDuplicates(%v)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		x, y     int
		expected int
	}{
		{name: "x greater than y", x: 5, y: 3, expected: 5},
		{name: "y greater than x", x: 2, y: 7, expected: 7},
		{name: "equal values", x: 4, y: 4, expected: 4},
		{name: "zero and positive", x: 0, y: 10, expected: 10},
		{name: "negative values", x: -3, y: -1, expected: -1},
		{name: "zero values", x: 0, y: 0, expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := max(tt.x, tt.y)
			if result != tt.expected {
				t.Errorf("max(%d, %d) = %d, want %d", tt.x, tt.y, result, tt.expected)
			}
		})
	}
}

func TestGetPort(t *testing.T) {
	t.Run("returns default port when env var not set", func(t *testing.T) {
		prev, had := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
		os.Unsetenv("FUNCTIONS_CUSTOMHANDLER_PORT")
		t.Cleanup(func() {
			if had {
				os.Setenv("FUNCTIONS_CUSTOMHANDLER_PORT", prev)
			} else {
				os.Unsetenv("FUNCTIONS_CUSTOMHANDLER_PORT")
			}
		})
		result := getPort()
		if result != ":8080" {
			t.Errorf("getPort() = %q, want %q", result, ":8080")
		}
	})

	t.Run("returns custom port from env var", func(t *testing.T) {
		t.Setenv("FUNCTIONS_CUSTOMHANDLER_PORT", "9090")
		result := getPort()
		if result != ":9090" {
			t.Errorf("getPort() = %q, want %q", result, ":9090")
		}
	})

	t.Run("prefixes colon to env var value", func(t *testing.T) {
		t.Setenv("FUNCTIONS_CUSTOMHANDLER_PORT", "3000")
		result := getPort()
		if result != ":3000" {
			t.Errorf("getPort() = %q, want %q", result, ":3000")
		}
	})
}
