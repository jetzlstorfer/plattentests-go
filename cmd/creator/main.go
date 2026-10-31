package creator

import (
	"context"
	"errors"
	"fmt"
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
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/kelseyhightower/envconfig"
)

// MaxSearchResults is the number of search results logged per query
const MaxSearchResults = 3

// MaxRecordsOfTheWeek is the maximum number of records of the week to be considered
const MaxRecordsOfTheWeek = 25

// MaxConcurrentSpotifySearches limits concurrent Spotify track lookups during playlist creation.
const MaxConcurrentSpotifySearches = 6

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

// Result contains playlist creation output records and unmatched tracks.
type Result struct {
	Records                 []crawler.Record
	NotFound                []string
	PlaylistID              string
	ShowFoundStatus         bool
	TotalTracks             int
	FoundTracks             int
	SearchSuccessRate       float64
	ComparedToProd          bool
	NewTracksComparedToProd int
	AlreadyInProdTracks     int
}

var playlistID spotify.ID

// CreatePlaylist builds the target Spotify playlist from Plattentests highlights.
func CreatePlaylist(pid string) (Result, error) {
	err := envconfig.Process("", &config)
	if err != nil {
		return Result{}, fmt.Errorf("load creator config: %w", err)
	}

	if pid == "" {
		playlistID = spotify.ID(os.Getenv("PLAYLIST_ID"))

	} else {
		playlistID = spotify.ID(pid)
	}

	log.Println("Plattentests.de Highlights of the week playlist generator")
	log.Println()

	if playlistID == "" || os.Getenv("SPOTIFY_ID") == "" || os.Getenv("SPOTIFY_SECRET") == "" {
		return Result{}, fmt.Errorf("PLAYLIST_ID, SPOTIFY_ID, or SPOTIFY_SECRET missing")
	}

	log.Println("Getting tracks of the week...")
	highlights, err := crawler.GetRecordsOfTheWeekSafe()
	if err != nil {
		return Result{}, fmt.Errorf("get records of the week: %w", err)
	}

	// only use the first records up to MAX_RECORDS_OF_THE_WEEK
	// this is mainly for debugging purposes
	if len(highlights) > MaxRecordsOfTheWeek {
		highlights = highlights[0:MaxRecordsOfTheWeek]
	}

	log.Println("Size of records of the week: ", len(highlights))

	log.Println("---")
	log.Println("Connecting to Spotify")
	log.Println("---")

	client, err := myauth.VerifyLogin()
	if err != nil {
		return Result{}, fmt.Errorf("spotify login failed: %w", err)
	}
	ctx := context.Background()

	log.Println("Emptying playlist...")
	err = client.ReplacePlaylistTracks(ctx, playlistID)
	if err != nil {
		return Result{}, fmt.Errorf("empty playlist: %w", err)
	}

	// put record of the week first, preserve original order for remaining records
	recordOfTheWeek, rotweErr := crawler.GetRecordOfTheWeekBandNameSafe()
	if rotweErr != nil {
		log.Printf("could not determine record of the week: %v", rotweErr)
	}
	highlights = orderRecordsForPlaylist(highlights, recordOfTheWeek)

	log.Println("Adding highlights of the week to playlist...")
	type highlightSearchJob struct {
		recordIdx int
		trackIdx  int
	}
	type highlightSearchResult struct {
		outcome searchOutcome
		err     error
	}

	jobs := make([]highlightSearchJob, 0)
	for i := range highlights {
		for j := range highlights[i].Tracks {
			if !highlights[i].Tracks[j].IsHighlight {
				continue
			}
			jobs = append(jobs, highlightSearchJob{recordIdx: i, trackIdx: j})
		}
	}

	total := len(jobs)
	var notFound []string

	// collect track IDs record by record, preserving within-record track order
	var itemsToAddIDs []spotify.ID
	for i := range highlights {
		record := &highlights[i]
		log.Println(record.Band + " - " + record.Recordname + ": " + record.Link)
	}

	if len(jobs) > 0 {
		results := make([]highlightSearchResult, len(jobs))
		jobIndexes := make(chan int)

		workerCount := MaxConcurrentSpotifySearches
		if len(jobs) < workerCount {
			workerCount = len(jobs)
		}

		var wg sync.WaitGroup
		wg.Add(workerCount)
		for w := 0; w < workerCount; w++ {
			go func() {
				defer wg.Done()
				for jobIdx := range jobIndexes {
					job := jobs[jobIdx]
					record := highlights[job.recordIdx]
					track := record.Tracks[job.trackIdx]
					outcome, searchErr := searchSong(ctx, &client, track.Trackname, record)
					results[jobIdx] = highlightSearchResult{outcome: outcome, err: searchErr}
				}
			}()
		}

		for i := range jobs {
			jobIndexes <- i
		}
		close(jobIndexes)
		wg.Wait()

		for i, job := range jobs {
			record := &highlights[job.recordIdx]
			track := &record.Tracks[job.trackIdx]
			result := results[i]

			if result.err != nil {
				log.Printf("search failed for %s - %s: %v", track.Band, track.Trackname, result.err)
				notFound = append(notFound, track.Band+" - "+track.Trackname)
				continue
			}

			if result.outcome.ID != "" {
				log.Println("adding item to collection to be added: " + result.outcome.ID)
				track.Found = true
				itemsToAddIDs = append(itemsToAddIDs, result.outcome.ID)
				continue
			}

			logMiss(*record, track.Trackname, result.outcome)
			notFound = append(notFound, track.Band+" - "+track.Trackname)
		}
	}

	// remove duplicates
	log.Println("removing duplicates...")
	noDuplicateTracks := removeDuplicates(itemsToAddIDs)

	// sort notfound tracks
	sort.Strings(notFound)

	// now add tracks to playlist
	log.Println("adding tracks to playlist...")
	if err := addTracks(client, noDuplicateTracks...); err != nil {
		return Result{}, err
	}

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

	foundTracks := total - len(notFound)
	result := Result{
		Records:           highlights,
		NotFound:          notFound,
		PlaylistID:        string(playlistID),
		ShowFoundStatus:   true,
		TotalTracks:       total,
		FoundTracks:       foundTracks,
		SearchSuccessRate: calculateSearchSuccessRate(foundTracks, total),
	}

	prodPlaylistID := strings.TrimSpace(os.Getenv("PLAYLIST_ID_PROD"))
	if prodPlaylistID != "" && spotify.ID(prodPlaylistID) != playlistID {
		prodTrackIDs, prodErr := getPlaylistTrackIDs(client, spotify.ID(prodPlaylistID))
		if prodErr != nil {
			log.Printf("could not compare against production playlist %s: %v", prodPlaylistID, prodErr)
		} else {
			newTracks, alreadyInProd := countNewTracksComparedToReference(noDuplicateTracks, prodTrackIDs)
			result.ComparedToProd = true
			result.NewTracksComparedToProd = newTracks
			result.AlreadyInProdTracks = alreadyInProd
		}
	}

	return result, nil
}

func orderRecordsForPlaylist(records []crawler.Record, recordOfTheWeek string) []crawler.Record {
	ordered := append([]crawler.Record(nil), records...)

	// Primary ordering: score descending.
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Score > ordered[j].Score
	})

	recordOfTheWeek = strings.TrimSpace(recordOfTheWeek)
	if recordOfTheWeek == "" {
		return ordered
	}

	for i := range ordered {
		if ordered[i].Band == recordOfTheWeek {
			ordered[i].IsRecordOfTheWeek = true
			break
		}
	}

	// Override ordering rule: record of the week always first.
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].IsRecordOfTheWeek && !ordered[j].IsRecordOfTheWeek
	})

	return ordered
}

// adds tracks to global playlist ID
func addTracks(client spotify.Client, trackids ...spotify.ID) error {
	if len(trackids) == 0 {
		log.Println("no tracks to add")
		return nil
	}
	_, err := client.AddTracksToPlaylist(context.Background(), playlistID, trackids...)
	if err != nil {
		return fmt.Errorf("could not add tracks to playlist: %w", err)
	}
	return nil

}

func getPlaylistTrackIDs(client spotify.Client, id spotify.ID) (map[spotify.ID]struct{}, error) {
	trackIDs := make(map[spotify.ID]struct{})
	page, err := client.GetPlaylistItems(context.Background(), id, spotify.Limit(100))
	if err != nil {
		return nil, fmt.Errorf("get playlist items for %s: %w", id, err)
	}

	for {
		for _, item := range page.Items {
			track := item.Track.Track
			if track == nil || track.ID == "" {
				continue
			}
			trackIDs[track.ID] = struct{}{}
		}

		if err := client.NextPage(context.Background(), page); err != nil {
			if errors.Is(err, spotify.ErrNoMorePages) {
				break
			}
			return nil, fmt.Errorf("paginate playlist %s: %w", id, err)
		}
	}

	return trackIDs, nil
}

// MarkFoundTracks marks each highlight track as found when it can be located on Spotify, using
// the same search and fuzzy-matching logic as playlist creation. Lookups run concurrently and are
// cached in-memory so listing pages can show a found indicator without repeating the searches on
// every request.
func MarkFoundTracks(records []crawler.Record) error {
	if len(records) == 0 {
		return nil
	}

	if err := envconfig.Process("", &config); err != nil {
		return fmt.Errorf("load creator config: %w", err)
	}

	client, err := myauth.VerifyLogin()
	if err != nil {
		return fmt.Errorf("spotify login failed: %w", err)
	}

	const maxConcurrentLookups = 8
	sem := make(chan struct{}, maxConcurrentLookups)
	var wg sync.WaitGroup

	for i := range records {
		record := records[i]
		for j := range records[i].Tracks {
			track := &records[i].Tracks[j]
			if !track.IsHighlight {
				continue
			}
			key := foundCacheKey(record.Band, track.Trackname)

			if found, ok := lookupFoundCache(key); ok {
				track.Found = found
				continue
			}

			wg.Add(1)
			sem <- struct{}{}
			go func(track *crawler.Track, record crawler.Record, key string) {
				defer wg.Done()
				defer func() { <-sem }()

				outcome, searchErr := searchSong(context.Background(), &client, track.Trackname, record)
				if searchErr != nil {
					log.Printf("found-status search failed for %s - %s: %v", record.Band, track.Trackname, searchErr)
					return
				}

				found := outcome.ID != ""
				storeFoundCache(key, found)
				track.Found = found
			}(track, record, key)
		}
	}

	wg.Wait()
	return nil
}

var (
	foundCacheMu sync.RWMutex
	foundCache   = make(map[string]bool)
)

func foundCacheKey(band, trackName string) string {
	return normalizeForComparison(band) + "\x00" + normalizeForComparison(trackName)
}

func lookupFoundCache(key string) (bool, bool) {
	foundCacheMu.RLock()
	defer foundCacheMu.RUnlock()
	found, ok := foundCache[key]
	return found, ok
}

func storeFoundCache(key string, found bool) {
	foundCacheMu.Lock()
	defer foundCacheMu.Unlock()
	foundCache[key] = found
}

func calculateSearchSuccessRate(found, total int) float64 {
	if total == 0 {
		return 0
	}
	return (float64(found) / float64(total)) * 100
}

func countNewTracksComparedToReference(candidates []spotify.ID, reference map[spotify.ID]struct{}) (int, int) {
	newTracks := 0
	alreadyKnown := 0

	for _, id := range candidates {
		if _, exists := reference[id]; exists {
			alreadyKnown++
			continue
		}
		newTracks++
	}

	return newTracks, alreadyKnown
}

var (
	// bracketFeaturePattern matches "(feat. X)", "[ft. X]", "(featuring X)" or "(with X)" and everything after it.
	bracketFeaturePattern = regexp.MustCompile(`(?i)\s*[\(\[]\s*(?:feat\.?|ft\.?|featuring|with)\s.*$`)
	// inlineFeaturePattern matches unbracketed "feat. X", "ft. X" or "featuring X" and everything after it.
	inlineFeaturePattern = regexp.MustCompile(`(?i)\s+(?:feat\.|ft\.|featuring)\s.*$`)
)

// removes parts of string that should not be in search term
func sanitizeTrackname(trackname string) string {
	sanitizedName := trackname

	// Remove featured-artist annotations and bonus markers
	sanitizedName = bracketFeaturePattern.ReplaceAllString(sanitizedName, "")
	sanitizedName = inlineFeaturePattern.ReplaceAllString(sanitizedName, "")
	sanitizedName = strings.Split(sanitizedName, "(Bonus)")[0]

	// Remove quotes and brackets
	sanitizedName = strings.ReplaceAll(sanitizedName, "\"", "")
	sanitizedName = strings.ReplaceAll(sanitizedName, "'", "")
	sanitizedName = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(sanitizedName, "")

	// Remove special punctuation that might interfere with search
	specialChars := regexp.MustCompile(`[:\-!?.,;]`)
	sanitizedName = specialChars.ReplaceAllString(sanitizedName, " ")

	// Normalize Unicode characters (remove accents/diacritics)
	sanitizedName = removeAccents(sanitizedName)

	// Clean up extra spaces
	sanitizedName = regexp.MustCompile(`\s+`).ReplaceAllString(sanitizedName, " ")
	sanitizedName = strings.TrimSpace(sanitizedName)

	return sanitizedName
}

func searchTermVariants(searchTerm string) []string {
	variants := []string{searchTerm}
	addVariant := func(candidate string) {
		for _, variant := range variants {
			if candidate == variant {
				return
			}
		}
		variants = append(variants, candidate)
	}

	if strings.Contains(searchTerm, "&") {
		addVariant(regexp.MustCompile(`\s*&\s*`).ReplaceAllString(searchTerm, " and "))
	}

	standaloneAnd := regexp.MustCompile(`(?i)\band\b`)
	if standaloneAnd.MatchString(searchTerm) {
		addVariant(standaloneAnd.ReplaceAllString(searchTerm, "&"))
	}

	return variants
}

// removeAccents removes accents and diacritics from Unicode characters
func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}

// normalizeForComparison normalizes a string for better comparison
func normalizeForComparison(s string) string {
	// Convert to lowercase and remove accents
	normalized := strings.ToLower(removeAccents(s))
	// Normalize & <-> "and" so both forms compare equal
	normalized = regexp.MustCompile(`\s*&\s*`).ReplaceAllString(normalized, " and ")
	// Remove common punctuation that might interfere with comparison
	specialChars := regexp.MustCompile(`[:\-!?.,;'"()]`)
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
	port := ":8081"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	return port
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}
