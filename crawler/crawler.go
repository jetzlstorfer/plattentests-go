package crawler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const url = "https://www.plattentests.de/index.php"
const baseurl = "https://www.plattentests.de/"

// GetTracksOfTheWeek return array of names for highlights of the week
func GetTracksOfTheWeek() []string {
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

	// TODO sort record collection

	var highlights []string
	fmt.Println("Size of recordsOfTheWeek: ", len(recordsOfTheWeek))
	for _, recordLink := range recordsOfTheWeek {
		fmt.Println(recordLink)
		highlights = append(highlights, getHighlights(recordLink)...)
	}
	return highlights

}

func getHighlights(recordLink string) []string {
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
	var highlights []string
	doc.Find("#rezihighlights li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		track := s.Text()
		fmt.Printf("Track %d: %s\n", i, bandname+" "+track)
		highlights = append(highlights, bandname+" "+track)
	})
	fmt.Println(len(highlights), " highlights found for", bandname)
	return highlights
}
