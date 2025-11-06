package creator

import (
	"context"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	myauth "github.com/jetzlstorfer/plattentests-go/internal/auth"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/kelseyhightower/envconfig"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// MaxSearchResults is the maximum number of search results to return
const MaxSearchResults = 3

// MaxRecordsOfTheWeek is the maximum number of records of the week to be considered
const MaxRecordsOfTheWeek = 25

var (
	config struct {
		TargetPlaylist  string `envconfig:"PLAYLIST_ID" required:"true"`
		ProdPlaylist    string `envconfig:"PROD_PLAYLIST_ID"`
		LogFile         string `envConfig:"LOG_FILE" required:"false"`
		TokenFile       string `envconfig:"TOKEN_FILE" required:"true"`
		AzAccountName   string `envconfig:"AZ_ACCOUNT" required:"true"`
		AzAccountKey    string `envconfig:"AZ_KEY" required:"true"`
		AzContainerName string `envconfig:"AZ_CONTAINER" required:"true"`
	}
)

type Result struct {
	Records    []crawler.Record
	NotFound   []string
	PlaylistID string
}

var playlistID spotify.ID

func CreatePlaylist(pid string) Result {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	if pid == "" {
		playlistID = spotify.ID(os.Getenv("PLAYLIST_ID"))

	} else {
		playlistID = spotify.ID(pid)
	}

	log.Println("Plattentests.de Highlights of the week playlist generator")
	log.Println()

	if playlistID == "" || os.Getenv("SPOTIFY_ID") == "" || os.Getenv("SPOTIFY_SECRET") == "" {
		log.Fatalln("PLAYLIST_ID, SPOTIFY_ID, or SPOTIFY_SECRET missing.")
	}

	log.Println("Getting tracks of the week...")
	highlights := crawler.GetRecordsOfTheWeek()

	// only use the first records up to MAX_RECORDS_OF_THE_WEEK
	// this is mainly for debugging purposes
	if len(highlights) > MaxRecordsOfTheWeek {
		highlights = highlights[0:MaxRecordsOfTheWeek]
	}

	log.Println("Size of records of the week: ", len(highlights))

	log.Println("---")
	log.Println("Connecting to Spotify")
	log.Println("---")

	// login to spotify, all error messages are dealt within the function
	client := myauth.VerifyLogin()
	ctx := context.Background()

	log.Println("Emptying playlist...")
	err = client.ReplacePlaylistTracks(ctx, playlistID)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Adding highlights of the week to playlist...")
	total := 0
	// var newTracks []spotify.ID
	var notFound []string
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup

	wg1.Add(len(highlights))

	type result struct {
		recordIndex int
		bandname    string
		itemID      spotify.ID
	}
	var itemsToAdd []result
	for i, record := range highlights {
		go func(i int, record crawler.Record) {
			defer wg1.Done()

			log.Println(record.Band + " - " + record.Recordname + ": " + record.Link)

			for j, track := range record.Tracks {
				wg2.Add(1)
				total++
				go func(j int, track crawler.Track) {
					defer wg2.Done()
					itemID := searchSong(client, track.Trackname, record)
					if itemID != "" {
						log.Println("adding item to collection to be added: " + itemID)
						// add new result to itemsToAdd
						r := result{itemID: itemID, bandname: record.Band, recordIndex: record.Score}
						itemsToAdd = append(itemsToAdd, r)
						record.Tracks[j].Tracklink = "https://open.spotify.com/track/" + itemID.String()
					} else {
						notFound = append(notFound, track.Band+" - "+track.Trackname)
					}
				}(j, track)
			}
			wg2.Wait()

		}(i, record)
	}
	wg1.Wait()

	// sort items by highest score and bandname
	sort.Slice(itemsToAdd[:], func(i, j int) bool {
		if itemsToAdd[i].recordIndex == itemsToAdd[j].recordIndex {
			return itemsToAdd[i].bandname < itemsToAdd[j].bandname
		}
		return itemsToAdd[i].recordIndex > itemsToAdd[j].recordIndex
	})

	// put record of the week on top of the playlist
	recordOfTheWeek := crawler.GetRecordOfTheWeekBandName()
	recordOfTheWeek, _ = charmap.ISO8859_1.NewDecoder().String(recordOfTheWeek)

	for _, item := range itemsToAdd {
		if item.bandname == recordOfTheWeek {
			itemsToAdd = append([]result{item}, itemsToAdd...)
		}
	}

	// extract spotify IDs from itemsToAdd
	var itemsToAddIDs []spotify.ID
	for _, item := range itemsToAdd {
		itemsToAddIDs = append(itemsToAddIDs, item.itemID)
	}

	// remove duplicates
	log.Println("removing duplicates...")
	noDuplicateTracks := removeDuplicates(itemsToAddIDs)

	// sort notfound tracks
	sort.Strings(notFound)

	// now add tracks to playlist
	log.Println("adding tracks to playlist...")
	addTracks(client, noDuplicateTracks...)

	log.Println()
	log.Println("--- RESULTS ---")
	log.Println()
	log.Println("total tracks:     ", total)
	log.Println("found tracks:     ", total-len(notFound))
	log.Println("not found tracks: ", len(notFound))
	log.Println()
	log.Println("Not found items: ")

	for _, track := range notFound {
		log.Println(" " + track)
	}

	// out some json with all records that should have been added and the once that have not been added
	outputJSON := make(map[string]interface{})
	outputJSON["highlights"] = highlights
	outputJSON["notFound"] = notFound

	return Result{
		Records:  highlights,
		NotFound: notFound,
	}
}

// selectBestTrack selects the best matching track from search results
// Priority:
// 1. If track name matches record name, prioritize that
// 2. Prefer album versions over singles/EPs
// 3. Use first result as fallback
func selectBestTrack(tracks []spotify.FullTrack, trackName string, record crawler.Record) *spotify.FullTrack {
	if len(tracks) == 0 {
		return nil
	}

	normalizedTrackName := normalizeForComparison(trackName)
	normalizedRecordName := normalizeForComparison(record.Recordname)

	type scoredTrack struct {
		track *spotify.FullTrack
		score int
	}

	var scored []scoredTrack

	for i := range tracks {
		track := &tracks[i]
		score := 0

		// Priority 1: Track name matches record name
		normalizedAlbumName := normalizeForComparison(track.Album.Name)
		if normalizedTrackName == normalizedRecordName && normalizedAlbumName == normalizedRecordName {
			score += 1000
			log.Printf(" [Priority] Track name '%s' matches record name '%s' on album '%s'", trackName, record.Recordname, track.Album.Name)
		}

		// Priority 2: Prefer album over single/EP
		if track.Album.AlbumType == "album" {
			score += 100
		} else if track.Album.AlbumType == "single" {
			score += 10
		}
		// EP gets no bonus (score += 0)

		// Priority 3: Earlier results get slight preference (all else being equal)
		score += (len(tracks) - i)

		scored = append(scored, scoredTrack{track: track, score: score})
		log.Printf(" [Score %d] %s - %s (%s) [%s]", score, track.Artists[0].Name, track.Name, track.Album.Name, track.Album.AlbumType)
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored[0].track
}

// searches a song given by the track and record name and returns spotify.ID if successful
func searchSong(client spotify.Client, track string, record crawler.Record) spotify.ID {
	searchTerm := sanitizeTrackname(record.Band + " " + track)
	// POTENTIAL FIX - do not include recordname in search
	//searchTerm = searchTerm + " " + record.Recordname

	// if record has a year, append it to the search
	if record.ReleaseYear != "" {
		searchTerm += " year:" + record.ReleaseYear
	}

	log.Printf(" searching term: %s", searchTerm)
	results, err := client.Search(context.Background(), searchTerm, spotify.SearchTypeTrack)
	if err != nil {
		log.Fatal(err)
	}
	// handle track results only if tracks are available
	if results.Tracks != nil && results.Tracks.Tracks != nil && len(results.Tracks.Tracks) > 0 {
		for i, item := range results.Tracks.Tracks {
			log.Printf(" found item: %s - %s  (%s) [%s]", item.Artists[0].Name, item.Name, item.Album.Name, item.Album.AlbumType)
			// only get MAX_SEARCH_RESULTS results
			if i >= MaxSearchResults-1 {
				break
			}
		}
		
		// Select best match from results with prioritization
		item := selectBestTrack(results.Tracks.Tracks, track, record)
		if item == nil {
			log.Printf(" no suitable match found after filtering")
			if record.Recordname == "" {
				return ""
			}
			log.Println(" nothing found, removing recordname and year from search query")
			newRecord := record
			newRecord.ReleaseYear = ""
			newRecord.Recordname = ""
			return searchSong(client, track, newRecord)
		}

		bandnameFromSearch := normalizeForComparison(item.Artists[0].Name)
		if len(item.Artists) > 1 {
			bandnameFromSearch += " " + normalizeForComparison(item.Artists[1].Name)
		}

		bandnameFromPlattentests := normalizeForComparison(record.Band)
		distance := levenshtein.DistanceForStrings([]rune(bandnameFromSearch), []rune(bandnameFromPlattentests), levenshtein.DefaultOptions)
		log.Println(" Levenshtein distance between", bandnameFromSearch, "and", bandnameFromPlattentests, ":", distance)
		threshold := 0.8

		calculatedThreshold := 1 - float64(distance)/float64(max(len(bandnameFromSearch), len(bandnameFromPlattentests)))
		if (calculatedThreshold) < threshold {
			log.Println(" Levenshtein distance too large")
			log.Printf(" not adding item %s - %s (%s) since artists don't match (%s != %s)", bandnameFromSearch, item.Name, item.Album.Name, bandnameFromPlattentests, bandnameFromSearch)
			if record.ReleaseYear == "" {
				return ""
			}
		}

		// calculate the levenshtein distance between the trackname from the search and the trackname from the record
		tracknameFromSearch := normalizeForComparison(item.Name)
		tracknameFromPlattentests := normalizeForComparison(track)
		distance = levenshtein.DistanceForStrings([]rune(tracknameFromSearch), []rune(tracknameFromPlattentests), levenshtein.DefaultOptions)

		calculatedThreshold = 1 - float64(distance)/float64(max(len(tracknameFromSearch), len(tracknameFromPlattentests)))
		if (calculatedThreshold) < threshold {
			log.Println(" Levenshtein distance too large")
			log.Printf(" not adding item %s - %s (%s) since tracknames don't match (%s != %s)", bandnameFromSearch, item.Name, item.Album.Name, tracknameFromPlattentests, tracknameFromSearch)
			if record.ReleaseYear == "" {
				return ""
			}
		}

		log.Printf(" using item: %s - %s (%s) [%s]", bandnameFromSearch, item.Name, item.Album.Name, item.Album.AlbumType)
		return item.ID
	}

	if record.Recordname == "" {
		log.Printf(" nothing found for %s", searchTerm)
		return ""
	}
	log.Println(" nothing found, removing recordname and year from search query")
	newRecord := record
	newRecord.ReleaseYear = ""
	newRecord.Recordname = ""
	return searchSong(client, track, newRecord)

}

// adds tracks to global playlist ID
func addTracks(client spotify.Client, trackids ...spotify.ID) bool {
	if len(trackids) == 0 {
		log.Println("no tracks to add")
		return false
	}
	_, err := client.AddTracksToPlaylist(context.Background(), playlistID, trackids...)
	if err != nil {
		log.Fatalf("could not add tracks to playlist: %s", err)
		return false
	}
	return true

}

// removes parts of string that should not be in search term
func sanitizeTrackname(trackname string) string {
	sanitizedName := trackname
	
	// Remove common patterns
	sanitizedName = strings.Split(sanitizedName, "(feat. ")[0]
	sanitizedName = strings.Split(sanitizedName, "(with ")[0]
	sanitizedName = strings.Split(sanitizedName, "(Bonus)")[0]
	
	// Remove quotes and brackets
	sanitizedName = strings.ReplaceAll(sanitizedName, "\"", "")
	sanitizedName = strings.ReplaceAll(sanitizedName, "'", "")
	sanitizedName = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(sanitizedName, "")
	
	// Remove special punctuation that might interfere with search
	specialChars := regexp.MustCompile(`[:\-&!?.,;]`)
	sanitizedName = specialChars.ReplaceAllString(sanitizedName, " ")
	
	// Normalize Unicode characters (remove accents/diacritics)
	sanitizedName = removeAccents(sanitizedName)
	
	// Clean up extra spaces
	sanitizedName = regexp.MustCompile(`\s+`).ReplaceAllString(sanitizedName, " ")
	sanitizedName = strings.TrimSpace(sanitizedName)
	
	return sanitizedName
}

// removeAccents removes accents and diacritics from Unicode characters
func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
	}), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}

// normalizeForComparison normalizes a string for better comparison
func normalizeForComparison(s string) string {
	// Convert to lowercase and remove accents
	normalized := strings.ToLower(removeAccents(s))
	// Remove common punctuation that might interfere with comparison
	specialChars := regexp.MustCompile(`[:\-&!?.,;'"()]`)
	normalized = specialChars.ReplaceAllString(normalized, " ")
	// Clean up extra spaces
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func removeDuplicates(sliceList []spotify.ID) []spotify.ID {
	allKeys := make(map[spotify.ID]bool)
	list := []spotify.ID{}
	for _, item := range sliceList {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func getPort() string {
	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	return port
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
