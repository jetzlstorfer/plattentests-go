package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Album represents an album's information
type Album struct {
	Image       string   `json:"Image"`
	Band        string   `json:"Band"`
	Recordname  string   `json:"Recordname"`
	Link        string   `json:"Link"`
	Score       int      `json:"Score"`
	ReleaseYear string   `json:"ReleaseYear"`
	Tracks      []string `json:"Tracks"`
}

type Highlight struct {
	Albums   []Album  `json:"Highlights"`
	NotFound []string `json:"NotFound"`
}

func main() {
	// Create a new Gin router
	r := gin.Default()

	// Load the template file
	tmpl, err := template.ParseFiles("templates/records.tmpl")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	// Load the template file
	tmpl2, err := template.ParseFiles("templates/createPlaylist.tmpl")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	// Define a handler function for the root endpoint
	r.GET("/", func(c *gin.Context) {
		// Fetch the album data from the given URL
		resp, err := http.Get("https://plattentests-go.azurewebsites.net/api/records/")
		if err != nil {
			log.Fatalf("Error fetching album data: %v", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		// Unmarshal the JSON data into an array of Album objects
		var albums []Album
		if err := json.Unmarshal(body, &albums); err != nil {
			log.Fatalf("Error unmarshaling album data: %v", err)
		}

		// Execute the template with the album data
		if err := tmpl.Execute(c.Writer, albums); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}
	})

	r.GET("/createPlaylist", func(c *gin.Context) {
		// Fetch the album data from the given URL
		resp, err := http.Get("https://plattentests-go.azurewebsites.net/api/createPlaylist/")
		if err != nil {
			log.Fatalf("Error fetching album data: %v", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		// Unmarshal the JSON data into an array of Album objects
		var highlights Highlight
		if err := json.Unmarshal(body, &highlights); err != nil {
			log.Fatalf("Error unmarshaling album data: %v", err)
		}

		// Execute the template with the album data
		if err := tmpl2.Execute(c.Writer, highlights); err != nil {
			log.Fatalf("Error executing template: %v", err)
		}
	})

	// Start the server
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
