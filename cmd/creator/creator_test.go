package creator

import (
	"os"
	"testing"

	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
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
			result := maxInt(tt.x, tt.y)
			if result != tt.expected {
				t.Errorf("maxInt(%d, %d) = %d, want %d", tt.x, tt.y, result, tt.expected)
			}
		})
	}
}

func TestGetPort(t *testing.T) {
	t.Run("returns default port when env var not set", func(t *testing.T) {
		prev, had := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
		if err := os.Unsetenv("FUNCTIONS_CUSTOMHANDLER_PORT"); err != nil {
			t.Fatalf("Unsetenv failed: %v", err)
		}
		t.Cleanup(func() {
			if had {
				if err := os.Setenv("FUNCTIONS_CUSTOMHANDLER_PORT", prev); err != nil {
					t.Errorf("Setenv cleanup failed: %v", err)
				}
			} else {
				if err := os.Unsetenv("FUNCTIONS_CUSTOMHANDLER_PORT"); err != nil {
					t.Errorf("Unsetenv cleanup failed: %v", err)
				}
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

func TestCalculateSearchSuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		found    int
		total    int
		expected float64
	}{
		{
			name:     "returns zero for empty total",
			found:    0,
			total:    0,
			expected: 0,
		},
		{
			name:     "returns full success rate",
			found:    4,
			total:    4,
			expected: 100,
		},
		{
			name:     "returns fractional success rate",
			found:    3,
			total:    4,
			expected: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSearchSuccessRate(tt.found, tt.total)
			if result != tt.expected {
				t.Errorf("calculateSearchSuccessRate(%d, %d) = %f, want %f", tt.found, tt.total, result, tt.expected)
			}
		})
	}
}

func TestCountNewTracksComparedToReference(t *testing.T) {
	tests := []struct {
		name             string
		candidates       []spotify.ID
		reference        map[spotify.ID]struct{}
		expectedNew      int
		expectedExisting int
	}{
		{
			name:             "all tracks are new",
			candidates:       []spotify.ID{"a", "b"},
			reference:        map[spotify.ID]struct{}{},
			expectedNew:      2,
			expectedExisting: 0,
		},
		{
			name:             "mixed new and existing tracks",
			candidates:       []spotify.ID{"a", "b", "c"},
			reference:        map[spotify.ID]struct{}{"b": {}, "d": {}},
			expectedNew:      2,
			expectedExisting: 1,
		},
		{
			name:             "all tracks already in comparison playlist",
			candidates:       []spotify.ID{"a", "b"},
			reference:        map[spotify.ID]struct{}{"a": {}, "b": {}},
			expectedNew:      0,
			expectedExisting: 2,
		},
	}

	func TestOrderRecordsForPlaylist(t *testing.T) {
		records := []crawler.Record{
			{
				Band:  "Band A",
				Score: 7,
				Tracks: []crawler.Track{
					{Trackname: "A1"},
					{Trackname: "A2"},
				},
			},
			{
				Band:  "Band B",
				Score: 9,
				Tracks: []crawler.Track{
					{Trackname: "B1"},
				},
			},
			{
				Band:  "Band C",
				Score: 9,
				Tracks: []crawler.Track{
					{Trackname: "C1"},
					{Trackname: "C2"},
				},
			},
		}

		ordered := orderRecordsForPlaylist(records, "Band A")

		if len(ordered) != 3 {
			t.Fatalf("expected 3 records, got %d", len(ordered))
		}

		if ordered[0].Band != "Band A" {
			t.Fatalf("record of the week must be first, got %s", ordered[0].Band)
		}

		if ordered[1].Band != "Band B" || ordered[2].Band != "Band C" {
			t.Fatalf("expected remaining records sorted by score while preserving order for equal scores, got [%s, %s]", ordered[1].Band, ordered[2].Band)
		}

		if !ordered[0].IsRecordOfTheWeek {
			t.Fatalf("expected first record to be marked as record of the week")
		}

		if len(ordered[0].Tracks) != 2 || ordered[0].Tracks[0].Trackname != "A1" || ordered[0].Tracks[1].Trackname != "A2" {
			t.Fatalf("track order within record must be preserved")
		}

		if records[0].Band != "Band A" || records[1].Band != "Band B" || records[2].Band != "Band C" {
			t.Fatalf("input slice must remain unchanged")
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTracks, existing := countNewTracksComparedToReference(tt.candidates, tt.reference)
			if newTracks != tt.expectedNew || existing != tt.expectedExisting {
				t.Errorf(
					"countNewTracksComparedToReference(%v, %v) = (%d, %d), want (%d, %d)",
					tt.candidates,
					tt.reference,
					newTracks,
					existing,
					tt.expectedNew,
					tt.expectedExisting,
				)
			}
		})
	}
}
