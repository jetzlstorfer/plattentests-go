package creator

import (
	"testing"

	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	"github.com/zmb3/spotify/v2"
)

func TestSanitizeTrackname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes feat.",
			input:    "Song Title (feat. Other Artist)",
			expected: "Song Title",
		},
		{
			name:     "removes with",
			input:    "Song Title (with Other Artist)",
			expected: "Song Title",
		},
		{
			name:     "removes Bonus",
			input:    "Song Title (Bonus)",
			expected: "Song Title",
		},
		{
			name:     "handles accented characters",
			input:    "Café Münchën",
			expected: "Cafe Munchen",
		},
		{
			name:     "handles special punctuation",
			input:    "Song: Title - With Dashes & Symbols!",
			expected: "Song Title With Dashes Symbols",
		},
		{
			name:     "handles unicode characters",
			input:    "Naïve Résumé",
			expected: "Naive Resume",
		},
		{
			name:     "handles mixed case and special chars",
			input:    "Artist Name - Song Title (feat. Other) & More",
			expected: "Artist Name Song Title",
		},
		{
			name:     "handles quotes and brackets",
			input:    "\"Song Title\" [Remix]",
			expected: "Song Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTrackname(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTrackname(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeForComparison(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "handles accented characters",
			input:    "Café Münchën",
			expected: "cafe munchen",
		},
		{
			name:     "handles special punctuation",
			input:    "Song: Title - With Dashes & Symbols!",
			expected: "song title with dashes symbols",
		},
		{
			name:     "handles unicode characters",
			input:    "Naïve Résumé",
			expected: "naive resume",
		},
		{
			name:     "handles mixed case",
			input:    "Artist NAME",
			expected: "artist name",
		},
		{
			name:     "handles quotes and parentheses",
			input:    "\"Song Title\" (Live)",
			expected: "song title live",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeForComparison(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeForComparison(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSelectBestTrack(t *testing.T) {
	tests := []struct {
		name           string
		tracks         []spotify.FullTrack
		trackName      string
		record         crawler.Record
		expectedIndex  int
		expectedReason string
	}{
		{
			name: "prioritizes track name matching record name on album",
			tracks: []spotify.FullTrack{
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-different-song",
						Name:    "Different Song",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Some Album",
						AlbumType: "album",
					},
				},
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-test-album-album",
						Name:    "Test Album",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Test Album",
						AlbumType: "album",
					},
				},
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-test-album-single",
						Name:    "Test Album",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Test Album",
						AlbumType: "single",
					},
				},
			},
			trackName: "Test Album",
			record: crawler.Record{
				Band:       "Artist",
				Recordname: "Test Album",
			},
			expectedIndex:  1,
			expectedReason: "track name matches record name on album version",
		},
		{
			name: "prefers album over single when track name does not match record name",
			tracks: []spotify.FullTrack{
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-song-single",
						Name:    "Song Title",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Greatest Hits",
						AlbumType: "single",
					},
				},
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-song-album",
						Name:    "Song Title",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Full Album",
						AlbumType: "album",
					},
				},
			},
			trackName: "Song Title",
			record: crawler.Record{
				Band:       "Artist",
				Recordname: "Full Album",
			},
			expectedIndex:  1,
			expectedReason: "album type preferred over single",
		},
		{
			name: "prefers album over EP",
			tracks: []spotify.FullTrack{
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-song-ep",
						Name:    "Song Title",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "EP Title",
						AlbumType: "ep",
					},
				},
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-song-album-2",
						Name:    "Song Title",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Album Title",
						AlbumType: "album",
					},
				},
			},
			trackName: "Song Title",
			record: crawler.Record{
				Band:       "Artist",
				Recordname: "Album Title",
			},
			expectedIndex:  1,
			expectedReason: "album type preferred over EP",
		},
		{
			name: "uses first result when all else is equal",
			tracks: []spotify.FullTrack{
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-album-one",
						Name:    "Song Title",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Album One",
						AlbumType: "album",
					},
				},
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-album-two",
						Name:    "Song Title",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Album Two",
						AlbumType: "album",
					},
				},
			},
			trackName: "Song Title",
			record: crawler.Record{
				Band:       "Artist",
				Recordname: "Different Album",
			},
			expectedIndex:  0,
			expectedReason: "first result used when all else is equal",
		},
		{
			name: "track name and record name match takes highest priority over album type",
			tracks: []spotify.FullTrack{
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-masterpiece-single",
						Name:    "Masterpiece",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Masterpiece",
						AlbumType: "single",
					},
				},
				{
					SimpleTrack: spotify.SimpleTrack{
						ID:      "id-other-song",
						Name:    "Other Song",
						Artists: []spotify.SimpleArtist{{Name: "Artist"}},
					},
					Album: spotify.SimpleAlbum{
						Name:      "Full Album",
						AlbumType: "album",
					},
				},
			},
			trackName: "Masterpiece",
			record: crawler.Record{
				Band:       "Artist",
				Recordname: "Masterpiece",
			},
			expectedIndex:  0,
			expectedReason: "track name matching record name has highest priority even for single",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectBestTrack(tt.tracks, tt.trackName, tt.record)
			if result == nil {
				t.Fatalf("selectBestTrack returned nil")
			}

			expectedID := tt.tracks[tt.expectedIndex].ID
			if result.ID != expectedID {
				// Find which track was selected for error reporting
				selectedIndex := -1
				for i := range tt.tracks {
					if tt.tracks[i].ID == result.ID {
						selectedIndex = i
						break
					}
				}
				t.Errorf("selectBestTrack selected track at index %d, want %d (%s)\nSelected: %s - %s (%s) [ID: %s]\nExpected: %s - %s (%s) [ID: %s]",
					selectedIndex, tt.expectedIndex, tt.expectedReason,
					result.Artists[0].Name, result.Name, result.Album.Name, result.ID,
					tt.tracks[tt.expectedIndex].Artists[0].Name,
					tt.tracks[tt.expectedIndex].Name,
					tt.tracks[tt.expectedIndex].Album.Name,
					expectedID,
				)
			}
		})
	}
}

func TestSelectBestTrack_EmptyTracks(t *testing.T) {
	result := selectBestTrack([]spotify.FullTrack{}, "track", crawler.Record{})
	if result != nil {
		t.Errorf("selectBestTrack should return nil for empty tracks, got %v", result)
	}
}