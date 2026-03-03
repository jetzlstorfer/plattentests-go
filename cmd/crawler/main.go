package crawler

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"github.com/jetzlstorfer/plattentests-go/internal/logging"
	"golang.org/x/text/encoding/charmap"
)

const plattentestsUrl = "https://www.plattentests.de/index.php"
const baseurl = "https://www.plattentests.de/"

// Record holds all information for a record
type Record struct {
	Image       string
	Band        string
	Recordname  string
	Link        string
	Score       int
	ReleaseYear string
	Tracks      []Track
	Description string
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
		logging.Fatal("Failed to fetch plattentests URL: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		logging.Fatal("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		logging.Fatal("Failed to parse HTML document: %v", err)
	}
	//logging.Debug(doc.Find("body").Text())

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
		logging.Fatal("Failed to fetch record link: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		logging.Fatal("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		logging.Fatal("Failed to parse HTML document: %v", err)
	}

	image := doc.Find(".headerbox img").First().AttrOr("src", "no image found")
	if image != "no image found" {
		image = baseurl + image
	}
	bandname := strings.Split(doc.Find("h1").Text(), " - ")[0]
	bandname, _ = charmap.ISO8859_1.NewDecoder().String(bandname)
	bandname = strings.Trim(bandname, " ")
	recordname := strings.Split(doc.Find("h1").Text(), " - ")[1]
	recordname, _ = charmap.ISO8859_1.NewDecoder().String(recordname)
	// for the releaseYear, find the following pattern ": dd.mm.yyyy"
	regex, _ := regexp.Compile(": [0-9]+.[0-9]+.[0-9]+")
	match := regex.FindString(doc.Find("p").Text())
	releaseYear := strings.Split(match, ".")[len(strings.Split(match, "."))-1]

	score, _ := strconv.Atoi(strings.Split(doc.Find("p.bewertung strong").First().Text(), "/")[0])

	// Extract description - the content follows h2 headings
	// The layout changed from .rezitext class to regular paragraphs after h2
	var paragraphs []string
	doc.Find("h2").Each(func(i int, h2 *goquery.Selection) {
		if i == 0 {
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
	description, err = charmap.ISO8859_1.NewDecoder().String(description)
	if err != nil {
		logging.Warn("Could not convert description to UTF8: %v", err)
	}

	var tracks []Track
	record := Record{image, bandname, recordname, recordLink, score, releaseYear, tracks, description}
	logging.Debug("%s - %s\n", bandname, recordname)
	doc.Find("#rezihighlights li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		track := Track{}
		trackname := s.Text()
		// only proceed if there are highlights available
		if strings.Trim(trackname, " ") != "-" {
			// decode into utf-8
			trackname, err = charmap.ISO8859_1.NewDecoder().String(trackname)
			if err != nil {
				logging.Warn("Could not convert trackname to UTF8: %v", err)
			}
			logging.Debug(" Track %d: %s\n", i+1, trackname)
			track.Band = bandname
			track.Trackname = trackname
			tracks = append(tracks, track)
		}
	})
	record.Tracks = tracks
	//logging.Debug("%d highlights found for %s", len(record.Tracks), record.Name)
	return record
}

// GetRecordOfTheWeek return name of record of the week
func GetRecordOfTheWeekBandName() string {
	// Request the HTML page.
	res, err := http.Get(plattentestsUrl)
	if err != nil {
		logging.Fatal("Failed to fetch plattentests URL: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		logging.Fatal("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		logging.Fatal("Failed to parse HTML document: %v", err)
	}

	return strings.Split(doc.Find("div.adw h3 a").Text(), " - ")[0]
}
