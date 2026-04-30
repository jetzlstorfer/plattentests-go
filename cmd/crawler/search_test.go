package crawler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
)

// mockSearchHTML returns a minimal Plattentests.de search results page that
// reproduces the structure of the real #suche container.
func mockSearchHTML(query string, interpreten, titel []struct{ Href, Title string }) string {
	render := func(items []struct{ Href, Title string }) string {
		if len(items) == 0 {
			return ""
		}
		var b strings.Builder
		b.WriteString("<ul>")
		for _, it := range items {
			b.WriteString(fmt.Sprintf(`<li><a href="%s">%s</a></li>`, it.Href, it.Title))
		}
		b.WriteString("</ul>")
		return b.String()
	}

	return fmt.Sprintf(`<html><body>
<div id="suche">
  <h2><span>Suche</span></h2>
  <p><strong>Du hast nach &quot;%s&quot; gesucht.</strong></p>
  <h3>Im Bereich &quot;Interpreten&quot; gab es %d Treffer</h3>
  %s
  <h3>Im Bereich &quot;Titel&quot; gab es %d Treffer</h3>
  %s
  <h3>Im Bereich &quot;Tracks&quot; gab es 1 Treffer</h3>
  <ul><li>Some Track / <a href="rezi.php?show=9999">Other Band - Other Album</a></li></ul>
  <h3>Im Bereich &quot;Forum&quot; gab es 1 Treffer</h3>
  <ul><li><a href="forum.php?topic=1">forum hit</a></li></ul>
</div>
</body></html>`, query, len(interpreten), render(interpreten), len(titel), render(titel))
}

func docFromString(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}
	return doc
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("invalid url %q: %v", s, err)
	}
	return u
}

func TestParseSearchResults_PicksInterpretenAndTitelOnly(t *testing.T) {
	html := mockSearchHTML("radiohead",
		[]struct{ Href, Title string }{
			{"rezi.php?show=3", "Radiohead - Kid A"},
			{"rezi.php?show=5278", "Radiohead - In rainbows"},
		},
		[]struct{ Href, Title string }{
			{"rezi.php?show=18199", "Radiohead - Kid A mnesia"},
		},
	)

	results := parseSearchResults(docFromString(t, html), mustURL(t, baseurl))

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	wantTitles := []string{"Radiohead - Kid A", "Radiohead - In rainbows", "Radiohead - Kid A mnesia"}
	for i, want := range wantTitles {
		if results[i].Title != want {
			t.Errorf("results[%d].Title = %q, want %q", i, results[i].Title, want)
		}
	}
	for _, r := range results {
		if !strings.HasPrefix(r.Link, baseurl+"rezi.php?show=") {
			t.Errorf("Link %q is not absolute rezi.php link", r.Link)
		}
	}
}

func TestParseSearchResults_DeduplicatesAcrossSections(t *testing.T) {
	// The same review can appear under both "Interpreten" and "Titel".
	html := mockSearchHTML("foo",
		[]struct{ Href, Title string }{{"rezi.php?show=42", "Band - Album"}},
		[]struct{ Href, Title string }{{"rezi.php?show=42", "Band - Album"}},
	)
	results := parseSearchResults(docFromString(t, html), mustURL(t, baseurl))
	if len(results) != 1 {
		t.Fatalf("expected 1 deduped result, got %d", len(results))
	}
}

func TestParseSearchResults_IgnoresNonReviewSections(t *testing.T) {
	// Tracks/Forum sections also contain rezi.php and other links but should
	// not be promoted to top-level search results.
	html := `<html><body><div id="suche">
<h3>Im Bereich &quot;Tracks&quot; gab es 1 Treffer</h3>
<ul><li>Track / <a href="rezi.php?show=1234">Some - Album</a></li></ul>
<h3>Im Bereich &quot;Forum&quot; gab es 1 Treffer</h3>
<ul><li><a href="forum.php?topic=1">forum hit</a></li></ul>
</div></body></html>`
	results := parseSearchResults(docFromString(t, html), mustURL(t, baseurl))
	if len(results) != 0 {
		t.Errorf("expected no results from non-review sections, got %d", len(results))
	}
}

func TestParseSearchResults_NoHits(t *testing.T) {
	html := `<html><body><div id="suche">
<h3>Im Bereich &quot;Interpreten&quot; gab es 0 Treffer</h3>
<h3>Im Bereich &quot;Titel&quot; gab es 0 Treffer</h3>
</div></body></html>`
	results := parseSearchResults(docFromString(t, html), mustURL(t, baseurl))
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

// fakePlattentestsServer simulates both the search endpoint and the per-record
// rezi.php pages so we can exercise the full Search() flow end-to-end.
func fakePlattentestsServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	var recordHits int
	mux := http.NewServeMux()

	// Mock search: returns two interpreten hits pointing back at this server.
	mux.HandleFunc("/suche.php", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		query := r.FormValue("suche")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><body><div id="suche">
<h3>Im Bereich &quot;Interpreten&quot; gab es 2 Treffer (%s)</h3>
<ul>
  <li><a href="rezi.php?show=1">Mock Band - First Album</a></li>
  <li><a href="rezi.php?show=2">Mock Band - Second Album</a></li>
</ul>
</div></body></html>`, query)
	})

	// Mock record pages.
	mux.HandleFunc("/rezi.php", func(w http.ResponseWriter, r *http.Request) {
		recordHits++
		show := r.URL.Query().Get("show")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><body>
<div class="headerbox"><img src="img/cover%s.jpg" /></div>
<h1>Mock Band - Album %s</h1>
<p>Veröffentlichung: 01.01.2024</p>
<p class="bewertung"><strong>8/10</strong></p>
<ul id="rezihighlights"><li>Track A</li></ul>
</body></html>`, show, show)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &recordHits
}

func TestSearch_FetchesEachHit(t *testing.T) {
	srv, hits := fakePlattentestsServer(t)

	records := searchAt(srv.URL+"/suche.php", "anything")

	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if *hits != 2 {
		t.Errorf("expected 2 record fetches, got %d", *hits)
	}
	// Order may differ because we fetch concurrently; check by membership.
	titles := map[string]bool{records[0].Recordname: true, records[1].Recordname: true}
	if !titles["Album 1"] || !titles["Album 2"] {
		t.Errorf("unexpected record names: %+v", titles)
	}
}

func TestSearch_EmptyQueryReturnsNil(t *testing.T) {
	if got := searchAt("http://unused", "   "); got != nil {
		t.Errorf("expected nil for empty query, got %v", got)
	}
}

func TestSearch_IgnoresNon200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if got := searchAt(srv.URL, "x"); got != nil {
		t.Errorf("expected nil on non-200 response, got %v", got)
	}
}

// Sanity check: the structure of a real-world style mock works with goquery.
func TestParseSearchResults_RealisticStructure(t *testing.T) {
	// Mirror the actual whitespace/newlines that Plattentests.de returns.
	html := `<div id="suche">
<h2><span>Suche</span></h2>
<p><strong> Du hast nach &quot;radiohead&quot; gesucht. </strong></p>
<h3>Im Bereich &quot;Interpreten&quot; gab es 2 Treffer</h3>
<ul>
    <li>
        <a href="rezi.php?show=3">Radiohead - Kid A</a>
    </li>
    <li>
        <a href="rezi.php?show=5278">Radiohead - In rainbows</a>
    </li>
</ul>
</div>`
	results := parseSearchResults(docFromString(t, html), mustURL(t, baseurl))
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}
func TestSearchRecords_ReturnsJSONForEmptyQuery(t *testing.T) {
gin.SetMode(gin.TestMode)
w := httptest.NewRecorder()
ctx, _ := gin.CreateTestContext(w)
ctx.Request = httptest.NewRequest("GET", "/search?q=", nil)

SearchRecords(ctx)

if w.Code != http.StatusOK {
t.Errorf("status = %d, want 200", w.Code)
}
// Empty query yields a nil/empty result, which marshals to "null" or "[]".
body := strings.TrimSpace(w.Body.String())
if body != "null" && body != "[]" {
t.Errorf("body = %q, want \"null\" or \"[]\"", body)
}
// Sanity-check: response is valid JSON.
var v interface{}
if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
t.Errorf("response is not valid JSON: %v", err)
}
}
