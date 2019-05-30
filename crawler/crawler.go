package crawler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const url = "https://www.plattentests.de/index.php"
const baseurl = "https://www.plattentests.de/"

// Record holds all information for a record
type Record struct {
	Name   string
	Link   string
	Score  int
	Tracks []string
}

// GetTracksOfTheWeek return array of names for highlights of the week
func GetTracksOfTheWeek() []Record {
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
	//fmt.Printf(doc.Find("body").Text())

	var recordsOfTheWeek []string
	// Find the review items
	doc.Find(".neuerezis li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		band := s.Find("a").Text()
		link, _ := s.Find("a").Attr("href")
		fmt.Printf("Review %d: %s - %s\n", i, band, link)
		recordsOfTheWeek = append(recordsOfTheWeek, baseurl+link)

	})

	var highlights []Record
	fmt.Println("Size of recordsOfTheWeek: ", len(recordsOfTheWeek))
	for _, recordLink := range recordsOfTheWeek {
		fmt.Println(recordLink)
		highlights = append(highlights, getHighlights(recordLink))
	}
	return highlights

}

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
	score, _ := strconv.Atoi(strings.Split(doc.Find("p.bewertung strong").First().Text(), "/")[0])

	var tracks []string
	record := Record{bandname, recordLink, score, tracks}
	doc.Find("#rezihighlights li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		track := s.Text()
		fmt.Printf("Track %d: %s\n", i, bandname+" "+track)
		tracks = append(tracks, bandname+" "+track)
	})
	record.Tracks = tracks
	fmt.Println(len(record.Tracks), " highlights found for", record.Name)
	return record
}
