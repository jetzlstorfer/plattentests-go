package crawler

import (
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/charmap"
)

const url = "https://www.plattentests.de/index.php"
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
}
type Track struct {
	Band      string
	Trackname string
	Tracklink string
}

// GetRecordsOfTheWeek return array of names for highlights of the week
func GetRecordsOfTheWeek() []Record {
	// Request the HTML page.
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
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

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	image := doc.Find(".headerbox img").First().AttrOr("src", "no image found")
	if image != "no image found" {
		image = baseurl + image
	}
	bandname := strings.Split(doc.Find("h1").Text(), " - ")[0]
	bandname, _ = charmap.ISO8859_1.NewDecoder().String(bandname)
	recordname := strings.Split(doc.Find("h1").Text(), " - ")[1]
	recordname, _ = charmap.ISO8859_1.NewDecoder().String(recordname)
	// for the releaseYear, find the following pattern ": dd.mm.yyyy"
	regex, _ := regexp.Compile(": [0-9]+.[0-9]+.[0-9]+")
	match := regex.FindString(doc.Find("p").Text())
	releaseYear := strings.Split(match, ".")[len(strings.Split(match, "."))-1]

	score, _ := strconv.Atoi(strings.Split(doc.Find("p.bewertung strong").First().Text(), "/")[0])

	var tracks []Track
	record := Record{image, bandname, recordname, recordLink, score, releaseYear, tracks}
	log.Printf("%s - %s\n", bandname, recordname)
	doc.Find("#rezihighlights li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		track := Track{}
		trackname := s.Text()
		// only proceed if there are highlights available
		if strings.Trim(trackname, " ") != "-" {
			// decode into utf-8
			trackname, err = charmap.ISO8859_1.NewDecoder().String(trackname)
			if err != nil {
				log.Printf("Could not convert trackname to UTF8: %v", err)
			}
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
func GetRecordOfTheWeek() string {
	// Request the HTML page.
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	return strings.Split(doc.Find("div.adw h3 a").Text(), " - ")[0]
}
