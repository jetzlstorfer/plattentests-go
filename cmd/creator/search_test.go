package creator

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	"github.com/zmb3/spotify/v2"
)

type fakeSearcher struct {
	mu           sync.Mutex
	trackResults map[string][]spotify.FullTrack
	allTracks    []spotify.FullTrack
	albumResults map[string][]spotify.SimpleAlbum
	albumTracks  map[spotify.ID][]spotify.SimpleTrack
	err          error
	queries      []string
}

func (f *fakeSearcher) Search(_ context.Context, query string, t spotify.SearchType, _ ...spotify.RequestOption) (*spotify.SearchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, query)
	if f.err != nil {
		return nil, f.err
	}
	if t == spotify.SearchTypeAlbum {
		return &spotify.SearchResult{Albums: &spotify.SimpleAlbumPage{Albums: f.albumResults[query]}}, nil
	}
	tracks, ok := f.trackResults[query]
	if !ok {
		tracks = f.allTracks
	}
	return &spotify.SearchResult{Tracks: &spotify.FullTrackPage{Tracks: tracks}}, nil
}

func (f *fakeSearcher) GetAlbumTracks(_ context.Context, id spotify.ID, _ ...spotify.RequestOption) (*spotify.SimpleTrackPage, error) {
	return &spotify.SimpleTrackPage{Tracks: f.albumTracks[id]}, nil
}

func artistsOf(names ...string) []spotify.SimpleArtist {
	artists := make([]spotify.SimpleArtist, 0, len(names))
	for _, name := range names {
		artists = append(artists, spotify.SimpleArtist{Name: name})
	}
	return artists
}

func fullTrack(id, name, album, albumType string, artists ...string) spotify.FullTrack {
	return spotify.FullTrack{
		SimpleTrack: spotify.SimpleTrack{ID: spotify.ID(id), Name: name, Artists: artistsOf(artists...)},
		Album:       spotify.SimpleAlbum{Name: album, AlbumType: albumType},
	}
}

func TestStripVersionInfo(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "Song - Remastered 2011", expected: "Song"},
		{input: "Song (2011 Remaster)", expected: "Song"},
		{input: "Song - Radio Edit", expected: "Song"},
		{input: "Song - Single Version", expected: "Song"},
		{input: "Song (feat. Someone)", expected: "Song"},
		{input: "Song [with Someone]", expected: "Song"},
		{input: "Song (Bonus Track)", expected: "Song"},
		{input: "Song (Live)", expected: "Song (Live)"},
		{input: "Song (Remix)", expected: "Song (Remix)"},
		{input: "Song (Clean Bandit Remix)", expected: "Song (Clean Bandit Remix)"},
		{input: "Song - From the Film", expected: "Song - From the Film"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if result := stripVersionInfo(tt.input); result != tt.expected {
				t.Errorf("stripVersionInfo(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTitlesMatch(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{name: "identical", a: "Song", b: "Song", expected: true},
		{name: "remaster suffix", a: "Song", b: "Song - Remastered 2011", expected: true},
		{name: "radio edit", a: "Song", b: "Song (Radio Edit)", expected: true},
		{name: "featured artist", a: "Song (feat. Guest)", b: "Song", expected: true},
		{name: "annotation on one side only", a: "Song", b: "Song (Live)", expected: true},
		{name: "ampersand and and", a: "Freaks & Geeks", b: "Freaks and Geeks", expected: true},
		{name: "accents", a: "Café", b: "Cafe", expected: true},
		{name: "small spelling difference", a: "Colour", b: "Color", expected: true},
		{name: "different parts in parentheses", a: "Part (1)", b: "Part (2)", expected: false},
		{name: "different numbers", a: "Song 1", b: "Song 2", expected: false},
		{name: "prefix of a longer title", a: "Love", b: "Love Me Do", expected: false},
		{name: "different song", a: "Song", b: "Another Tune", expected: false},
		{name: "empty title", a: "", b: "Song", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := titlesMatch(tt.a, tt.b); result != tt.expected {
				t.Errorf("titlesMatch(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestArtistsMatch(t *testing.T) {
	tests := []struct {
		name     string
		artists  []string
		band     string
		expected bool
	}{
		{name: "exact", artists: []string{"Muff Potter"}, band: "Muff Potter", expected: true},
		{name: "leading the", artists: []string{"Beatles"}, band: "The Beatles", expected: true},
		{name: "ampersand and and", artists: []string{"Simon & Garfunkel"}, band: "Simon and Garfunkel", expected: true},
		{name: "collaboration listed separately", artists: []string{"Thees Uhlmann", "Marcus Wiebusch"}, band: "Thees Uhlmann & Marcus Wiebusch", expected: true},
		{name: "featured artist in band name", artists: []string{"Casper", "Marteria"}, band: "Casper feat. Marteria", expected: true},
		{name: "only primary artist credited", artists: []string{"Thees Uhlmann"}, band: "Thees Uhlmann & Marcus Wiebusch", expected: true},
		{name: "band credited after guest", artists: []string{"Guest", "Main Band"}, band: "Main Band", expected: true},
		{name: "diacritics", artists: []string{"Mötley Crüe"}, band: "Motley Crue", expected: true},
		{name: "different artist", artists: []string{"Someone Else"}, band: "Muff Potter", expected: false},
		{name: "different primary artist", artists: []string{"Kettcar"}, band: "Thees Uhlmann & Marcus Wiebusch", expected: false},
		{name: "no artists", artists: nil, band: "Muff Potter", expected: false},
		{name: "empty band", artists: []string{"Muff Potter"}, band: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := artistsMatch(artistsOf(tt.artists...), tt.band); result != tt.expected {
				t.Errorf("artistsMatch(%q, %q) = %v, want %v", tt.artists, tt.band, result, tt.expected)
			}
		})
	}
}

func TestYearFilter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "2026", expected: "year:2025-2026"},
		{input: " 2024 ", expected: "year:2023-2024"},
		{input: "", expected: ""},
		{input: "unknown", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if result := yearFilter(tt.input); result != tt.expected {
				t.Errorf("yearFilter(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTrackSearchQueries(t *testing.T) {
	tests := []struct {
		name     string
		band     string
		track    string
		year     string
		expected []string
	}{
		{
			name:  "field filters first, then free text, year range before no year",
			band:  "Muff Potter",
			track: "Freaks & Geeks",
			year:  "2026",
			expected: []string{
				`track:"Freaks & Geeks" artist:"Muff Potter" year:2025-2026`,
				`track:"Freaks and Geeks" artist:"Muff Potter" year:2025-2026`,
				`Muff Potter Freaks & Geeks year:2025-2026`,
				`Muff Potter Freaks and Geeks year:2025-2026`,
				`track:"Freaks & Geeks" artist:"Muff Potter"`,
				`track:"Freaks and Geeks" artist:"Muff Potter"`,
				`Muff Potter Freaks & Geeks`,
				`Muff Potter Freaks and Geeks`,
			},
		},
		{
			name:  "sanitizes band and track separately",
			band:  "Artist feat. Guest",
			track: "Song (feat. Other)",
			expected: []string{
				`track:"Song" artist:"Artist"`,
				`Artist Song`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trackSearchQueries(tt.band, tt.track, tt.year)
			if len(result) != len(tt.expected) {
				t.Fatalf("trackSearchQueries() = %q, want %q", result, tt.expected)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("trackSearchQueries()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSelectBestTrack_MatchingRules(t *testing.T) {
	tests := []struct {
		name       string
		tracks     []spotify.FullTrack
		trackName  string
		record     crawler.Record
		expectedID spotify.ID
	}{
		{
			name: "skips wrong-artist album ranked first",
			tracks: []spotify.FullTrack{
				fullTrack("wrong", "Song", "Other Album", "album", "Cover Band"),
				fullTrack("right", "Song", "Song", "single", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "right",
		},
		{
			name: "returns nil when no candidate matches the artist",
			tracks: []spotify.FullTrack{
				fullTrack("wrong", "Song", "Album", "album", "Cover Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "",
		},
		{
			name: "returns nil when no candidate matches the title",
			tracks: []spotify.FullTrack{
				fullTrack("wrong", "Another Tune", "Album", "album", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "",
		},
		{
			name: "prefers the reviewed record over another album",
			tracks: []spotify.FullTrack{
				fullTrack("hits", "Song", "Greatest Hits", "album", "Band"),
				fullTrack("record", "Song", "The EP", "single", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band", Recordname: "The EP"},
			expectedID: "record",
		},
		{
			name: "prefers exact title over annotated version",
			tracks: []spotify.FullTrack{
				fullTrack("live", "Song (Live)", "Album", "album", "Band"),
				fullTrack("studio", "Song - Remastered 2020", "Album", "album", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "studio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectBestTrack(tt.tracks, tt.trackName, tt.record)
			var resultID spotify.ID
			if result != nil {
				resultID = result.ID
			}
			if resultID != tt.expectedID {
				t.Errorf("selectBestTrack() = %q, want %q", resultID, tt.expectedID)
			}
		})
	}
}

func TestSearchSong(t *testing.T) {
	tests := []struct {
		name            string
		searcher        *fakeSearcher
		track           string
		record          crawler.Record
		expectedID      spotify.ID
		expectedReason  missReason
		expectedQueries int
		expectErr       bool
	}{
		{
			name: "does not accept a mismatched artist when a release year is set",
			searcher: &fakeSearcher{
				allTracks: []spotify.FullTrack{fullTrack("wrong", "Song", "Album", "album", "Cover Band")},
			},
			track:           "Song",
			record:          crawler.Record{Band: "Band", ReleaseYear: "2026"},
			expectedReason:  missArtistMismatch,
			expectedQueries: 4,
		},
		{
			name: "falls back to the query without year",
			searcher: &fakeSearcher{
				trackResults: map[string][]spotify.FullTrack{
					`track:"Song" artist:"Band"`: {fullTrack("found", "Song", "Album", "album", "Band")},
				},
			},
			track:           "Song",
			record:          crawler.Record{Band: "Band", ReleaseYear: "2026"},
			expectedID:      "found",
			expectedQueries: 3,
		},
		{
			name: "matches within the reviewed album tracklist",
			searcher: &fakeSearcher{
				albumResults: map[string][]spotify.SimpleAlbum{
					`album:"The Record" artist:"Band"`: {
						{ID: "search-test-other-album", Name: "The Record", Artists: artistsOf("Cover Band")},
						{ID: "search-test-album", Name: "The Record (Deluxe Edition)", Artists: artistsOf("Band")},
					},
				},
				albumTracks: map[spotify.ID][]spotify.SimpleTrack{
					"search-test-other-album": {{ID: "cover", Name: "Song"}},
					"search-test-album": {
						{ID: "intro", Name: "Intro"},
						{ID: "from-album", Name: "Song - Remastered 2024"},
					},
				},
			},
			track:           "Song",
			record:          crawler.Record{Band: "Band", Recordname: "The Record"},
			expectedID:      "from-album",
			expectedQueries: 3,
		},
		{
			name: "reports a title mismatch",
			searcher: &fakeSearcher{
				allTracks: []spotify.FullTrack{fullTrack("other", "Another Tune", "Album", "album", "Band")},
			},
			track:           "Song",
			record:          crawler.Record{Band: "Band"},
			expectedReason:  missTitleMismatch,
			expectedQueries: 2,
		},
		{
			name:            "reports no results",
			searcher:        &fakeSearcher{},
			track:           "Song",
			record:          crawler.Record{Band: "Band", Recordname: "The Record"},
			expectedReason:  missNoResults,
			expectedQueries: 4,
		},
		{
			name:      "returns search errors",
			searcher:  &fakeSearcher{err: errors.New("boom")},
			track:     "Song",
			record:    crawler.Record{Band: "Band"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome, err := searchSong(context.Background(), tt.searcher, tt.track, tt.record)
			if tt.expectErr {
				if err == nil {
					t.Fatal("searchSong() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("searchSong() error = %v", err)
			}
			if outcome.ID != tt.expectedID {
				t.Errorf("searchSong() ID = %q, want %q", outcome.ID, tt.expectedID)
			}
			if outcome.Reason != tt.expectedReason {
				t.Errorf("searchSong() Reason = %q, want %q", outcome.Reason, tt.expectedReason)
			}
			if len(outcome.Queries) != tt.expectedQueries || len(tt.searcher.queries) != tt.expectedQueries {
				t.Errorf("searchSong() queries = %q (sent %q), want %d", outcome.Queries, tt.searcher.queries, tt.expectedQueries)
			}
		})
	}
}

func TestSearchSong_SendsLimitAndMarket(t *testing.T) {
	var mu sync.Mutex
	var path string
	var params url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		path = r.URL.Path
		params = r.URL.Query()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"tracks":{"items":[{"id":"abc","name":"Song","artists":[{"name":"Band"}],"album":{"name":"Record","album_type":"album"}}]}}`)
	}))
	defer server.Close()

	client := spotify.New(server.Client(), spotify.WithBaseURL(server.URL+"/"))
	outcome, err := searchSong(context.Background(), client, "Song", crawler.Record{Band: "Band", Recordname: "Record"})
	if err != nil {
		t.Fatalf("searchSong() error = %v", err)
	}
	if outcome.ID != "abc" {
		t.Fatalf("searchSong() ID = %q, want %q", outcome.ID, "abc")
	}

	mu.Lock()
	defer mu.Unlock()
	if path != "/search" {
		t.Errorf("request path = %q, want %q", path, "/search")
	}
	expected := map[string]string{
		"q":      `track:"Song" artist:"Band"`,
		"type":   "track",
		"limit":  "10",
		"market": "DE",
	}
	for key, want := range expected {
		if got := params.Get(key); got != want {
			t.Errorf("query parameter %s = %q, want %q", key, got, want)
		}
	}
}
