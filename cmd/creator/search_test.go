package creator

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	"github.com/zmb3/spotify/v2"
)

type fakeSearcher struct {
	trackResults map[string][]spotify.FullTrack
	allTracks    []spotify.FullTrack
	albumResults map[string][]spotify.SimpleAlbum
	albumTracks  map[spotify.ID][]spotify.SimpleTrack
	queries      []string
}

func (f *fakeSearcher) Search(_ context.Context, query string, t spotify.SearchType, _ ...spotify.RequestOption) (*spotify.SearchResult, error) {
	f.queries = append(f.queries, query)
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

func resetAlbumTracksCache(t *testing.T) {
	t.Helper()
	albumTracksCacheMu.Lock()
	original := albumTracksCache
	albumTracksCache = make(map[spotify.ID][]spotify.SimpleTrack)
	albumTracksCacheMu.Unlock()
	t.Cleanup(func() {
		albumTracksCacheMu.Lock()
		albumTracksCache = original
		albumTracksCacheMu.Unlock()
	})
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
		{name: "inline featured artist", a: "Song feat. Guest", b: "Song", expected: true},
		{name: "inline ft", a: "Song ft. Guest", b: "Song", expected: true},
		{name: "inline featuring", a: "Song featuring Guest", b: "Song", expected: true},
		{name: "annotation on one side only", a: "Song", b: "Song (Live)", expected: true},
		{name: "ampersand and and", a: "Freaks & Geeks", b: "Freaks and Geeks", expected: true},
		{name: "accents", a: "Café", b: "Cafe", expected: true},
		{name: "small spelling difference", a: "Colour", b: "Color", expected: true},
		{name: "different parts in parentheses", a: "Part (1)", b: "Part (2)", expected: false},
		{name: "different numbers", a: "Song 1", b: "Song 2", expected: false},
		{name: "Arabic and Roman numeral", a: "Godfather 2", b: "Godfather II", expected: true},
		{name: "lowercase Roman numeral", a: "Godfather ii", b: "Godfather 2", expected: true},
		{name: "different Roman numbered parts", a: "Song II", b: "Song III", expected: false},
		{name: "numbered annotation is significant", a: "Song (Part 2)", b: "Song", expected: false},
		{name: "numbered dash suffix is significant", a: "Song - Part 2", b: "Song", expected: false},
		{name: "similarity at threshold", a: "abcde", b: "abcdx", expected: true},
		{name: "similarity below threshold", a: "abcd", b: "abcx", expected: false},
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
		{input: "0", expected: ""},
		{input: "1", expected: ""},
		{input: "-2026", expected: ""},
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
			if !slices.Equal(result, tt.expected) {
				t.Errorf("trackSearchQueries() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSelectBestTrack(t *testing.T) {
	tests := []struct {
		name       string
		tracks     []spotify.FullTrack
		trackName  string
		record     crawler.Record
		expectedID spotify.ID
	}{
		{name: "empty results"},
		{
			name: "prefers album over single independently of record name",
			tracks: []spotify.FullTrack{
				fullTrack("single", "Song", "Single", "single", "Band"),
				fullTrack("album", "Song", "Album", "album", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "album",
		},
		{
			name: "prefers album over EP independently of record name",
			tracks: []spotify.FullTrack{
				fullTrack("ep", "Song", "EP", "ep", "Band"),
				fullTrack("album", "Song", "Album", "album", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "album",
		},
		{
			name: "preserves Spotify order for equal matches",
			tracks: []spotify.FullTrack{
				fullTrack("first", "Song", "First Album", "album", "Band"),
				fullTrack("second", "Song", "Second Album", "album", "Band"),
			},
			trackName:  "Song",
			record:     crawler.Record{Band: "Band"},
			expectedID: "first",
		},
		{
			name: "title track matches reviewed album",
			tracks: []spotify.FullTrack{
				fullTrack("wrong", "Other Song", "Album", "album", "Band"),
				fullTrack("album", "Masterpiece", "Masterpiece", "album", "Band"),
				fullTrack("single", "Masterpiece", "Masterpiece", "single", "Band"),
			},
			trackName:  "Masterpiece",
			record:     crawler.Record{Band: "Band", Recordname: "Masterpiece"},
			expectedID: "album",
		},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAlbumTracksCache(t)
			outcome, err := searchSong(context.Background(), tt.searcher, tt.track, tt.record)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("request path = %q, want /search", r.URL.Path)
		}
		expected := map[string]string{
			"q":      `track:"Song" artist:"Band"`,
			"type":   "track",
			"limit":  "10",
			"market": "DE",
		}
		for key, want := range expected {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("query parameter %s = %q, want %q", key, got, want)
			}
		}
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

}

func TestBestTracklistMatch(t *testing.T) {
	tracks := []spotify.SimpleTrack{
		{ID: "fuzzy", Name: "Song (Live)"},
		{ID: "exact", Name: "Song - Remastered 2020"},
	}
	for _, tt := range []struct {
		name   string
		tracks []spotify.SimpleTrack
		want   spotify.ID
	}{
		{name: "exact before fuzzy", tracks: tracks, want: "exact"},
		{name: "fuzzy fallback", tracks: tracks[:1], want: "fuzzy"},
		{name: "no match", tracks: []spotify.SimpleTrack{{ID: "other", Name: "Another Tune"}}},
		{name: "empty tracklist"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			match := bestTracklistMatch(tt.tracks, "Song")
			var got spotify.ID
			if match != nil {
				got = match.ID
			}
			if got != tt.want {
				t.Errorf("bestTracklistMatch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearchSong_AlbumPaginationAndCache(t *testing.T) {
	for _, tt := range []struct {
		name           string
		failSecondPage bool
		wantRequests   int32
	}{
		{name: "reads and caches all pages", wantRequests: 2},
		{name: "does not cache incomplete tracklists", failSecondPage: true, wantRequests: 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAlbumTracksCache(t)
			var tracklistRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if got := r.URL.Query().Get("market"); got != "DE" {
					t.Errorf("market = %q, want DE", got)
				}
				if r.URL.Path == "/search" {
					if got := r.URL.Query().Get("limit"); got != "10" {
						t.Errorf("search limit = %q, want 10", got)
					}
					if r.URL.Query().Get("type") == "album" {
						_, _ = io.WriteString(w, `{"albums":{"items":[{"id":"album","name":"Record","artists":[{"name":"Band"}]}]}}`)
					} else {
						_, _ = io.WriteString(w, `{"tracks":{"items":[]}}`)
					}
					return
				}
				if r.URL.Path != "/albums/album/tracks" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				requestNumber := tracklistRequests.Add(1)
				if got := r.URL.Query().Get("limit"); got != "50" {
					t.Errorf("album tracks limit = %q, want 50", got)
				}
				var page spotify.SimpleTrackPage
				switch offset := r.URL.Query().Get("offset"); offset {
				case "", "0":
					page.Tracks = make([]spotify.SimpleTrack, 50)
					for i := range page.Tracks {
						page.Tracks[i] = spotify.SimpleTrack{ID: "intro", Name: "Intro"}
					}
					page.Next = "https://api.spotify.com/v1/albums/album/tracks?offset=50"
				case "50":
					if tt.failSecondPage && requestNumber == 2 {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = io.WriteString(w, `{"error":{"status":500,"message":"page failure"}}`)
						return
					}
					page.Tracks = []spotify.SimpleTrack{{ID: "last-track", Name: "Song"}}
				default:
					t.Errorf("unexpected offset: %q", offset)
				}
				if err := json.NewEncoder(w).Encode(page); err != nil {
					t.Errorf("encode tracks: %v", err)
				}
			}))
			defer server.Close()
			client := spotify.New(server.Client(), spotify.WithBaseURL(server.URL+"/"))
			record := crawler.Record{Band: "Band", Recordname: "Record"}
			if tt.failSecondPage {
				outcome, err := searchSong(context.Background(), client, "Song", record)
				if err == nil || !strings.Contains(err.Error(), "page failure") || outcome.ID != "" {
					t.Fatalf("searchSong() = %q, %v; want page failure without a match", outcome.ID, err)
				}
			}
			for _, lookup := range []struct {
				title string
				want  spotify.ID
			}{{"Song", "last-track"}, {"Intro", "intro"}} {
				outcome, err := searchSong(context.Background(), client, lookup.title, record)
				if err != nil || outcome.ID != lookup.want {
					t.Fatalf("searchSong(%q) = %q, %v; want %q, nil", lookup.title, outcome.ID, err, lookup.want)
				}
			}
			if got := tracklistRequests.Load(); got != tt.wantRequests {
				t.Errorf("tracklist requests = %d, want %d", got, tt.wantRequests)
			}
		})
	}
}

func TestSearchSong_Errors(t *testing.T) {
	for _, tt := range []struct {
		phase  string
		prefix string
	}{
		{phase: "track search", prefix: "search "},
		{phase: "album search", prefix: "search album "},
		{phase: "album tracks", prefix: "get tracks of album "},
	} {
		t.Run(tt.phase, func(t *testing.T) {
			resetAlbumTracksCache(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/search" && r.URL.Query().Get("type") == "track" && tt.phase != "track search":
					_, _ = io.WriteString(w, `{"tracks":{"items":[]}}`)
				case r.URL.Path == "/search" && r.URL.Query().Get("type") == "album" && tt.phase == "album tracks":
					_, _ = io.WriteString(w, `{"albums":{"items":[{"id":"album","name":"Record","artists":[{"name":"Band"}]}]}}`)
				default:
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, `{"error":{"status":500,"message":"upstream failure"}}`)
				}
			}))
			defer server.Close()
			client := spotify.New(server.Client(), spotify.WithBaseURL(server.URL+"/"))
			outcome, err := searchSong(context.Background(), client, "Song", crawler.Record{Band: "Band", Recordname: "Record"})
			if err == nil || !strings.HasPrefix(err.Error(), tt.prefix) || !strings.Contains(err.Error(), "upstream failure") {
				t.Fatalf("searchSong() error = %v, want upstream failure with context %q", err, tt.prefix)
			}
			if outcome.ID != "" {
				t.Errorf("searchSong() returned ID %q on failure", outcome.ID)
			}
		})
	}
}

func TestSearchSong_RateLimit(t *testing.T) {
	for _, tt := range []struct {
		name       string
		retryAfter string
		timeout    time.Duration
		wantErr    error
		wantCalls  int32
	}{
		{name: "retries then succeeds", retryAfter: "0", timeout: 5 * time.Second, wantCalls: 2},
		{name: "deadline interrupts retry wait", retryAfter: "60", timeout: 250 * time.Millisecond, wantErr: context.DeadlineExceeded, wantCalls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", tt.retryAfter)
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, `{"error":{"status":429,"message":"rate limited"}}`)
					return
				}
				_, _ = io.WriteString(w, `{"tracks":{"items":[{"id":"found","name":"Song","artists":[{"name":"Band"}] }]}}`)
			}))
			defer server.Close()
			client := spotify.New(server.Client(), spotify.WithBaseURL(server.URL+"/"), spotify.WithRetry(true))
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			outcome, err := searchSong(ctx, client, "Song", crawler.Record{Band: "Band"})
			if tt.wantErr == nil && err != nil {
				t.Fatalf("searchSong() error = %v, want nil", err)
			}
			// The SDK returns the last 429 error when cancellation interrupts its retry wait.
			if tt.wantErr != nil && (err == nil || !errors.Is(ctx.Err(), tt.wantErr)) {
				t.Fatalf("searchSong() error = %v, context error = %v; want a failed lookup after %v", err, ctx.Err(), tt.wantErr)
			}
			if err == nil && outcome.ID != "found" {
				t.Errorf("searchSong() ID = %q, want found", outcome.ID)
			}
			if got := calls.Load(); got != tt.wantCalls {
				t.Errorf("requests = %d, want %d", got, tt.wantCalls)
			}
		})
	}
}
