package crawler

import (
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html/charset"
)

const plattentestsUrl = "https://www.plattentests.de/index.php"
const baseurl = "https://www.plattentests.de/"
const searchUrl = "https://www.plattentests.de/suche.php"

// maxSearchResults limits how many record pages we fetch for a single search
// query to avoid hammering Plattentests.de when a query matches a lot of
// reviews. The native search may return hundreds of matches.
const maxSearchResults = 25

func newDocumentFromPlattentestsResponse(res *http.Response) (*goquery.Document, error) {
	decodedReader, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(decodedReader)
}

// Record holds all information for a record
type Record struct {
	Image             string
	Band              string
	Recordname        string
	Link              string
	Score             int
	ReleaseYear       string
	Tracks            []Track
	Headline          string
	Description       string
	IsRecordOfTheWeek bool
}
type Track struct {
	Band      string
	Trackname string
	Tracklink string
}

// GetRecordsOfTheWeek return array of names for highlights of the week
func GetRecordsOfTheWeek() []Record {
	// Request the HTML page.
	res, err := http.Get(plattentestsUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Plattentests uses ISO-8859-1; decode before parsing to preserve umlauts/special chars.
	doc, err := newDocumentFromPlattentestsResponse(res)
	if err != nil {
		log.Fatal(err)
	}
	//log.Printf(doc.Find("body").Text())

	var highlights []Record
	// Find the review items
	newReviews := doc.Find(".neuerezis li")

	var wg sync.WaitGroup
	wg.Add(newReviews.Length())

	newReviews.Each(func(i int, s *goquery.Selection) {

		go func(i int, s *goquery.Selection) {
			defer wg.Done()
			// For each item found, get the link
			link, _ := s.Find("a").Attr("href")
			highlights = append(highlights, getHighlightsByRecordLink(baseurl+link))
		}(i, s)

	})

	wg.Wait()

	// sort record collection
	sort.Slice(highlights[:], func(i, j int) bool {
		return strings.Compare(highlights[i].Band, highlights[j].Band) <= 0
	})

	return highlights
}

func PrintRecordsOfTheWeek(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, GetRecordsOfTheWeek())
}

func GetRecord(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid record identifier")
		return
	}
	recordUrl := "rezi.php?show="
	recordLink := baseurl + recordUrl + strconv.Itoa(id)
	c.IndentedJSON(http.StatusOK, getHighlightsByRecordLink(recordLink))
}

// getting highlights of a particular record by recordLink
func getHighlightsByRecordLink(recordLink string) Record {
	res, err := http.Get(recordLink)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Plattentests uses ISO-8859-1; decode before parsing to preserve umlauts/special chars.
	doc, err := newDocumentFromPlattentestsResponse(res)
	if err != nil {
		log.Fatal(err)
	}

	image := doc.Find(".headerbox img").First().AttrOr("src", "no image found")
	if image != "no image found" {
		image = baseurl + image
	}
	bandname := strings.Split(doc.Find("h1").Text(), " - ")[0]
	bandname = strings.Trim(bandname, " ")
	recordname := strings.Split(doc.Find("h1").Text(), " - ")[1]
	// for the releaseYear, find the following pattern ": dd.mm.yyyy"
	regex, _ := regexp.Compile(": [0-9]+.[0-9]+.[0-9]+")
	match := regex.FindString(doc.Find("p").Text())
	releaseYear := strings.Split(match, ".")[len(strings.Split(match, "."))-1]

	score, _ := strconv.Atoi(strings.Split(doc.Find("p.bewertung strong").First().Text(), "/")[0])

	// Extract headline and description - the content follows h2 headings
	// The layout changed from .rezitext class to regular paragraphs after h2
	var headline string
	var paragraphs []string
	doc.Find("h2").Each(func(i int, h2 *goquery.Selection) {
		if i == 0 {
			headline = strings.TrimSpace(h2.Text())
			// Get siblings after h2 that are paragraphs, until next h2/h3/h4
			h2.NextUntil("h2, h3, h4, hr").Each(func(j int, elem *goquery.Selection) {
				if goquery.NodeName(elem) == "p" {
					pText := strings.TrimSpace(elem.Text())
					// Skip very short paragraphs (metadata) and footer/nav text
					if len(pText) > 100 && !strings.Contains(pText, "Startseite") && !strings.Contains(pText, "Referenzen") {
						paragraphs = append(paragraphs, pText)
					}
				}
			})
		}
	})

	description := strings.Join(paragraphs, " ")

	var tracks []Track
	record := Record{
		Image:       image,
		Band:        bandname,
		Recordname:  recordname,
		Link:        recordLink,
		Score:       score,
		ReleaseYear: releaseYear,
		Tracks:      tracks,
		Headline:    headline,
		Description: description,
	}
	log.Printf("%s - %s\n", bandname, recordname)
	doc.Find("#rezihighlights li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		track := Track{}
		trackname := s.Text()
		// only proceed if there are highlights available
		if strings.Trim(trackname, " ") != "-" {
			log.Printf(" Track %d: %s\n", i+1, trackname)
			track.Band = bandname
			track.Trackname = trackname
			tracks = append(tracks, track)
		}
	})
	record.Tracks = tracks
	//log.Println(len(record.Tracks), " highlights found for", record.Name)
	return record
}

// GetRecordOfTheWeek return name of record of the week
func GetRecordOfTheWeekBandName() string {
	// Request the HTML page.
	res, err := http.Get(plattentestsUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Plattentests uses ISO-8859-1; decode before parsing to preserve umlauts/special chars.
	doc, err := newDocumentFromPlattentestsResponse(res)
	if err != nil {
		log.Fatal(err)
	}

	return strings.Split(doc.Find("div.adw h3 a").Text(), " - ")[0]
}

// SearchResult represents a single hit from the Plattentests.de search page,
// pointing at an album review. It is the lightweight intermediate type we
// produce while parsing the search HTML, before fetching the full record.
type SearchResult struct {
	Title string // Display title from the link, e.g. "Radiohead - Kid A"
	Link  string // Absolute URL to the rezi.php page
}

// Search queries Plattentests.de for the given term and returns the matching
// album reviews as fully populated Records (band, title, score, tracks, …).
//
// The native search at https://www.plattentests.de/suche.php is a POST form
// that returns multiple result sections (Interpreten, Titel, Tracks,
// Referenzen, Autor, Specials, Forum). We only consider the album-review
// sections ("Interpreten" and "Titel") because the other sections either
// link to non-review pages or duplicate the review hits.
//
// Results are capped at maxSearchResults to limit load on Plattentests.de.
func Search(query string) []Record {
	return searchAt(searchUrl, query)
}

// searchAt is the testable variant of Search that allows pointing at a
// different (e.g. mocked) base URL.
func searchAt(endpoint, query string) []Record {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	// Resolve relative rezi.php links against the search endpoint so tests
	// (and any future deployment behind a different host) work correctly.
	base, err := url.Parse(endpoint)
	if err != nil {
		log.Printf("invalid search endpoint %q: %v", endpoint, err)
		return nil
	}

	form := url.Values{}
	form.Set("suche", query)
	form.Set("parameter", "all")

	res, err := http.PostForm(endpoint, form)
	if err != nil {
		log.Printf("search request failed: %v", err)
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("search status code error: %d %s", res.StatusCode, res.Status)
		return nil
	}

	doc, err := newDocumentFromPlattentestsResponse(res)
	if err != nil {
		log.Printf("failed to parse search response: %v", err)
		return nil
	}

	hits := parseSearchResults(doc, base)
	if len(hits) > maxSearchResults {
		hits = hits[:maxSearchResults]
	}

	records := make([]Record, len(hits))
	var wg sync.WaitGroup
	wg.Add(len(hits))
	for i, hit := range hits {
		go func(i int, link string) {
			defer wg.Done()
			records[i] = getHighlightsByRecordLink(link)
		}(i, hit.Link)
	}
	wg.Wait()

	return records
}

// parseSearchResults extracts album-review hits from a parsed Plattentests.de
// search results page. It walks the h3 section headings inside the #suche
// container and only collects rezi.php links from the "Interpreten" and
// "Titel" sections; duplicates are removed (the same album can match both).
// Relative hrefs are resolved against base.
func parseSearchResults(doc *goquery.Document, base *url.URL) []SearchResult {
	var results []SearchResult
	seen := make(map[string]bool)

	doc.Find("#suche h3").Each(func(_ int, h *goquery.Selection) {
		text := h.Text()
		if !strings.Contains(text, "Interpreten") && !strings.Contains(text, "Titel") {
			return
		}
		// The matching <ul> is the next sibling element after the h3.
		h.NextFiltered("ul").Find("a").Each(func(_ int, a *goquery.Selection) {
			href, ok := a.Attr("href")
			if !ok {
				return
			}
			if !strings.HasPrefix(href, "rezi.php?show=") {
				return
			}
			if seen[href] {
				return
			}
			seen[href] = true

			absolute := href
			if base != nil {
				if u, err := base.Parse(href); err == nil {
					absolute = u.String()
				}
			}
			results = append(results, SearchResult{
				Title: strings.TrimSpace(a.Text()),
				Link:  absolute,
			})
		})
	})

	return results
}

// SearchRecords is a Gin handler that exposes Search as a JSON endpoint.
// It expects the search term in the "q" query parameter.
func SearchRecords(c *gin.Context) {
	query := c.Query("q")
	c.IndentedJSON(http.StatusOK, Search(query))
}
