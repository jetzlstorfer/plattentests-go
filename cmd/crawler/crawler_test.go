package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockHTML returns a minimal HTML page that matches the selectors used by getHighlightsByRecordLink.
func mockRecordHTML(band, record, imageHref, score, date string, tracks []string, headline, description string) string {
	trackItems := ""
	for _, t := range tracks {
		trackItems += fmt.Sprintf("<li>%s</li>\n", t)
	}

	imgTag := ""
	if imageHref != "" {
		imgTag = fmt.Sprintf(`<div class="headerbox"><img src="%s" /></div>`, imageHref)
	}

	h2Section := ""
	if headline != "" {
		h2Section = fmt.Sprintf("<h2>%s</h2>\n<p>%s</p>\n", headline, description)
	}

	return fmt.Sprintf(`<html><body>
%s
<h1>%s - %s</h1>
<p>Veröffentlichung%s </p>
<p class="bewertung"><strong>%s</strong></p>
%s
<ul id="rezihighlights">
%s
</ul>
</body></html>`, imgTag, band, record, date, score, h2Section, trackItems)
}

func startMockServer(t *testing.T, html string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, html)
	}))
}

func TestGetHighlightsByRecordLink_BasicRecord(t *testing.T) {
	html := mockRecordHTML(
		"Test Band",
		"Test Album",
		"img/cover.jpg",
		"8/10",
		": 15.03.2024",
		[]string{"Track One", "Track Two"},
		"A Great Headline",
		"This is a longer description paragraph that exceeds one hundred characters. It describes the album well.",
	)

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Band != "Test Band" {
		t.Errorf("Band = %q, want %q", rec.Band, "Test Band")
	}
	if rec.Recordname != "Test Album" {
		t.Errorf("Recordname = %q, want %q", rec.Recordname, "Test Album")
	}
	if rec.Score != 8 {
		t.Errorf("Score = %d, want 8", rec.Score)
	}
	if rec.ReleaseYear != "2024" {
		t.Errorf("ReleaseYear = %q, want %q", rec.ReleaseYear, "2024")
	}
	if len(rec.Tracks) != 2 {
		t.Errorf("len(Tracks) = %d, want 2", len(rec.Tracks))
	}
	if rec.Tracks[0].Trackname != "Track One" {
		t.Errorf("Tracks[0].Trackname = %q, want %q", rec.Tracks[0].Trackname, "Track One")
	}
	if rec.Tracks[1].Trackname != "Track Two" {
		t.Errorf("Tracks[1].Trackname = %q, want %q", rec.Tracks[1].Trackname, "Track Two")
	}
	// Band field is set on each track
	for i, track := range rec.Tracks {
		if track.Band != "Test Band" {
			t.Errorf("Tracks[%d].Band = %q, want %q", i, track.Band, "Test Band")
		}
	}
}

func TestGetHighlightsByRecordLink_ImageURL(t *testing.T) {
	html := mockRecordHTML(
		"Image Band", "Image Album",
		"img/album_cover.jpg",
		"7/10", ": 01.01.2023",
		nil, "", "",
	)
	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	expectedImage := baseurl + "img/album_cover.jpg"
	if rec.Image != expectedImage {
		t.Errorf("Image = %q, want %q", rec.Image, expectedImage)
	}
}

func TestGetHighlightsByRecordLink_NoImage(t *testing.T) {
	// No .headerbox img element present
	html := `<html><body>
<h1>No Image Band - No Image Album</h1>
<p>Release: 01.01.2023</p>
<p class="bewertung"><strong>6/10</strong></p>
</body></html>`

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Image != "no image found" {
		t.Errorf("Image = %q, want %q", rec.Image, "no image found")
	}
}

func TestGetHighlightsByRecordLink_DashTracksAreSkipped(t *testing.T) {
	html := mockRecordHTML(
		"Dash Band", "Dash Album",
		"", "5/10", ": 10.06.2023",
		[]string{"-", "Real Track", " - "},
		"", "",
	)

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	// Only "Real Track" should appear; dash entries are skipped
	if len(rec.Tracks) != 1 {
		t.Errorf("len(Tracks) = %d, want 1", len(rec.Tracks))
	}
	if len(rec.Tracks) > 0 && rec.Tracks[0].Trackname != "Real Track" {
		t.Errorf("Tracks[0].Trackname = %q, want %q", rec.Tracks[0].Trackname, "Real Track")
	}
}

func TestGetHighlightsByRecordLink_NoTracks(t *testing.T) {
	html := mockRecordHTML(
		"No Track Band", "No Track Album",
		"", "4/10", ": 01.01.2022",
		nil, "", "",
	)

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if len(rec.Tracks) != 0 {
		t.Errorf("expected 0 tracks, got %d", len(rec.Tracks))
	}
}

func TestGetHighlightsByRecordLink_HeadlineAndDescription(t *testing.T) {
	longDesc := strings.Repeat("This is a great album description. ", 4) // >100 chars
	html := mockRecordHTML(
		"Headline Band", "Headline Album",
		"", "9/10", ": 05.05.2023",
		nil,
		"An Epic Headline",
		longDesc,
	)

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Headline != "An Epic Headline" {
		t.Errorf("Headline = %q, want %q", rec.Headline, "An Epic Headline")
	}
	if !strings.Contains(rec.Description, "great album description") {
		t.Errorf("Description does not contain expected text, got %q", rec.Description)
	}
}

func TestGetHighlightsByRecordLink_ShortDescriptionSkipped(t *testing.T) {
	// Short paragraph (<= 100 chars) should not be included in description
	html := fmt.Sprintf(`<html><body>
<h1>Short Desc Band - Short Desc Album</h1>
<p>Release: 01.01.2023</p>
<p class="bewertung"><strong>7/10</strong></p>
<h2>Some Headline</h2>
<p>Short.</p>
</body></html>`)

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Description != "" {
		t.Errorf("Description should be empty for short paragraphs, got %q", rec.Description)
	}
}

func TestGetHighlightsByRecordLink_DescriptionExcludesNavText(t *testing.T) {
	// Paragraphs containing "Startseite" or "Referenzen" should be excluded
	longNavText := strings.Repeat("Startseite navigation filler text here. ", 4)
	html := mockRecordHTML(
		"Nav Band", "Nav Album",
		"", "6/10", ": 01.01.2023",
		nil,
		"Some Headline",
		longNavText,
	)

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Description != "" {
		t.Errorf("Description should exclude nav text, got %q", rec.Description)
	}
}

func TestGetHighlightsByRecordLink_NoH2NoHeadline(t *testing.T) {
	html := `<html><body>
<h1>Band - Album</h1>
<p>Release: 01.01.2023</p>
<p class="bewertung"><strong>7/10</strong></p>
</body></html>`

	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Headline != "" {
		t.Errorf("Headline should be empty when no h2 present, got %q", rec.Headline)
	}
	if rec.Description != "" {
		t.Errorf("Description should be empty when no h2 present, got %q", rec.Description)
	}
}

func TestGetHighlightsByRecordLink_RecordLink(t *testing.T) {
	html := mockRecordHTML(
		"Link Band", "Link Album",
		"", "7/10", ": 01.01.2023",
		nil, "", "",
	)
	srv := startMockServer(t, html)
	defer srv.Close()

	rec := getHighlightsByRecordLink(srv.URL)

	if rec.Link != srv.URL {
		t.Errorf("Link = %q, want %q", rec.Link, srv.URL)
	}
}

func TestNewDocumentFromPlattentestsResponse_UTF8(t *testing.T) {
	// Verify that UTF-8 content passes through without corruption
	input := "<html><body><p>Motörhead &amp; Björk</p></body></html>"
	res := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:   io.NopCloser(strings.NewReader(input)),
	}

	doc, err := newDocumentFromPlattentestsResponse(res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := doc.Text()
	if !strings.Contains(text, "Motörhead") {
		t.Errorf("expected text to contain %q, got %q", "Motörhead", text)
	}
	if !strings.Contains(text, "Björk") {
		t.Errorf("expected text to contain %q, got %q", "Björk", text)
	}
}
