package crawler // to be defined if "main" or "crawler"

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/charmap"
)

const url = "https://www.plattentests.de/index.php"
const baseurl = "https://www.plattentests.de/"

// Record holds all information for a record
type Record struct {
	Band        string
	Recordname  string
	Link        string
	Score       int
	ReleaseYear string
	Tracks      []string
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
	doc.Find(".neuerezis li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		recordTitle := s.Find("a").Text()
		recordTitle, _ = charmap.ISO8859_1.NewDecoder().String(recordTitle)
		link, _ := s.Find("a").Attr("href")
		//log.Printf("Review %d: %s - %s\n", i, band, link)
		log.Println(recordTitle)
		highlights = append(highlights, getHighlights(baseurl+link))

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
	c.IndentedJSON(http.StatusOK, getHighlights(recordLink))
}

// getting highlights of a particular record by recordLink
func getHighlights(recordLink string) Record {
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

	bandname := strings.Split(doc.Find("h1").Text(), " - ")[0]
	bandname, _ = charmap.ISO8859_1.NewDecoder().String(bandname)
	recordname := strings.Split(doc.Find("h1").Text(), " - ")[1]
	recordname, _ = charmap.ISO8859_1.NewDecoder().String(recordname)
	// for the releaseYear, find the following pattern ": dd.mm.yyyy"
	regex, _ := regexp.Compile(": [0-9]+.[0-9]+.[0-9]+")
	match := regex.FindString(doc.Find("p").Text())
	releaseYear := strings.Split(match, ".")[len(strings.Split(match, "."))-1]

	score, _ := strconv.Atoi(strings.Split(doc.Find("p.bewertung strong").First().Text(), "/")[0])

	var tracks []string
	record := Record{bandname, recordname, recordLink, score, releaseYear, tracks}
	doc.Find("#rezihighlights li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		track := s.Text()
		// only proceed if there are highlights available
		if strings.Trim(track, " ") != "-" {
			// decoce into utf-8
			track, err = charmap.ISO8859_1.NewDecoder().String(track)
			if err != nil {
				log.Printf("Could not convert trackname to UTF8: %v", err)
			}
			log.Printf(" Track %d: %s\n", i+1, bandname+" "+track)
			tracks = append(tracks, bandname+" "+track)
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
