package creator

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/agnivade/levenshtein"
	crawler "github.com/jetzlstorfer/plattentests-go/cmd/crawler"
	"github.com/zmb3/spotify/v2"
)

const (
	// spotifySearchLimit is the maximum search page size Spotify allows for development-mode apps.
	spotifySearchLimit = 10
	// spotifyAlbumTracksLimit is the maximum page size of the album tracklist endpoint.
	spotifyAlbumTracksLimit = 50
	// searchMarket restricts results to tracks playable in Germany, Plattentests' home market.
	searchMarket = "DE"
	// similarityThreshold is the minimum normalized Levenshtein similarity for two names to match.
	similarityThreshold = 0.8
	// searchTimeout bounds a single track lookup, including rate-limit retries.
	searchTimeout = 90 * time.Second
)

// spotifySearcher is the subset of the Spotify client used for track lookups.
type spotifySearcher interface {
	Search(ctx context.Context, query string, t spotify.SearchType, opts ...spotify.RequestOption) (*spotify.SearchResult, error)
	GetAlbumTracks(ctx context.Context, id spotify.ID, opts ...spotify.RequestOption) (*spotify.SimpleTrackPage, error)
}

type missReason string

const (
	missNoResults      missReason = "no results"
	missArtistMismatch missReason = "artist mismatch"
	missTitleMismatch  missReason = "title mismatch"
)

// searchOutcome describes a track lookup. ID is empty when nothing matched; Reason then explains why.
type searchOutcome struct {
	ID      spotify.ID
	Reason  missReason
	Queries []string
}

type missDiagnostics struct {
	sawResults     bool
	sawArtistMatch bool
}

func (d *missDiagnostics) observeTracks(tracks []spotify.FullTrack, band string) {
	d.sawResults = true
	for i := range tracks {
		if artistsMatch(tracks[i].Artists, band) {
			d.sawArtistMatch = true
			return
		}
	}
}

func (d missDiagnostics) reason() missReason {
	switch {
	case !d.sawResults:
		return missNoResults
	case d.sawArtistMatch:
		return missTitleMismatch
	default:
		return missArtistMismatch
	}
}

// searchSong looks up a Plattentests highlight on Spotify. It tries field-filtered and free-text
// queries (with and without a release-year range) and only accepts candidates whose artist and
// title match. As a last resort it searches the reviewed album and matches within its tracklist.
func searchSong(ctx context.Context, client spotifySearcher, track string, record crawler.Record) (searchOutcome, error) {
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	var outcome searchOutcome
	var diag missDiagnostics

	for _, query := range trackSearchQueries(record.Band, track, record.ReleaseYear) {
		outcome.Queries = append(outcome.Queries, query)
		log.Printf(" searching term: %s", query)
		results, err := client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(spotifySearchLimit), spotify.Market(searchMarket))
		if err != nil {
			return outcome, fmt.Errorf("search %q: %w", query, err)
		}
		if results == nil || results.Tracks == nil || len(results.Tracks.Tracks) == 0 {
			continue
		}

		candidates := results.Tracks.Tracks
		logTrackResults(candidates)
		diag.observeTracks(candidates, record.Band)

		if item := selectBestTrack(candidates, track, record); item != nil {
			log.Printf(" using item: %s - %s (%s) [%s]", joinArtistNames(item.Artists), item.Name, item.Album.Name, item.Album.AlbumType)
			outcome.ID = item.ID
			return outcome, nil
		}
	}

	if strings.TrimSpace(record.Recordname) != "" {
		id, err := searchAlbumTracklist(ctx, client, track, record, &outcome, &diag)
		if err != nil {
			return outcome, err
		}
		if id != "" {
			outcome.ID = id
			return outcome, nil
		}
	}

	outcome.Reason = diag.reason()
	return outcome, nil
}

// trackSearchQueries builds the ordered list of track search queries: field-filtered first, then
// free text, each with the release-year range before falling back to no year filter.
func trackSearchQueries(band, track, releaseYear string) []string {
	cleanBand := sanitizeTrackname(band)
	cleanTrack := sanitizeTrackname(track)

	var bases []string
	if cleanBand != "" && cleanTrack != "" {
		bases = append(bases, fmt.Sprintf(`track:"%s" artist:"%s"`, cleanTrack, cleanBand))
	}
	if freeText := strings.TrimSpace(cleanBand + " " + cleanTrack); freeText != "" {
		bases = append(bases, freeText)
	}

	var filters []string
	if filter := yearFilter(releaseYear); filter != "" {
		filters = append(filters, " "+filter)
	}
	filters = append(filters, "")

	var queries []string
	for _, filter := range filters {
		for _, base := range bases {
			queries = appendUnique(queries, searchTermVariants(base+filter)...)
		}
	}
	return queries
}

// yearFilter returns a Spotify year filter covering the release year and the year before,
// since singles are often released ahead of the album. Non-numeric years yield no filter.
func yearFilter(releaseYear string) string {
	year, err := strconv.Atoi(strings.TrimSpace(releaseYear))
	if err != nil || year <= 1 {
		return ""
	}
	return fmt.Sprintf("year:%d-%d", year-1, year)
}

// selectBestTrack selects the best matching track from search results.
// Candidates must match the Plattentests artist and track title. Remaining candidates are ranked by:
// 1. the album name matching the reviewed record,
// 2. album releases over singles over EPs/compilations,
// 3. an exact (version-insensitive) title match,
// 4. the original search result order.
func selectBestTrack(tracks []spotify.FullTrack, trackName string, record crawler.Record) *spotify.FullTrack {
	type scoredTrack struct {
		track *spotify.FullTrack
		score int
	}

	wantedTitle := comparableTitle(trackName)
	var scored []scoredTrack

	for i := range tracks {
		track := &tracks[i]
		if !artistsMatch(track.Artists, record.Band) {
			log.Printf(" [Skip] %s - %s: artist does not match %q", joinArtistNames(track.Artists), track.Name, record.Band)
			continue
		}
		if !titlesMatch(track.Name, trackName) {
			log.Printf(" [Skip] %s - %s: title does not match %q", joinArtistNames(track.Artists), track.Name, trackName)
			continue
		}

		score := 0
		if strings.TrimSpace(record.Recordname) != "" && titlesMatch(track.Album.Name, record.Recordname) {
			score += 500
		}
		switch track.Album.AlbumType {
		case "album":
			score += 100
		case "single":
			score += 10
		}
		if comparableTitle(track.Name) == wantedTitle {
			score += 50
		}
		score += len(tracks) - i

		scored = append(scored, scoredTrack{track: track, score: score})
		log.Printf(" [Score %d] %s - %s (%s) [%s]", score, joinArtistNames(track.Artists), track.Name, track.Album.Name, track.Album.AlbumType)
	}

	if len(scored) == 0 {
		return nil
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	return scored[0].track
}

// searchAlbumTracklist searches the reviewed album and matches the track within its tracklist.
func searchAlbumTracklist(ctx context.Context, client spotifySearcher, track string, record crawler.Record, outcome *searchOutcome, diag *missDiagnostics) (spotify.ID, error) {
	cleanBand := sanitizeTrackname(record.Band)
	cleanRecord := sanitizeTrackname(record.Recordname)
	if cleanRecord == "" {
		return "", nil
	}

	var queries []string
	if cleanBand != "" {
		queries = appendUnique(queries, searchTermVariants(fmt.Sprintf(`album:"%s" artist:"%s"`, cleanRecord, cleanBand))...)
	}
	queries = appendUnique(queries, searchTermVariants(strings.TrimSpace(cleanBand+" "+cleanRecord))...)

	checked := make(map[spotify.ID]bool)
	for _, query := range queries {
		outcome.Queries = append(outcome.Queries, "[album] "+query)
		log.Printf(" searching album: %s", query)
		results, err := client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(spotifySearchLimit), spotify.Market(searchMarket))
		if err != nil {
			return "", fmt.Errorf("search album %q: %w", query, err)
		}
		if results == nil || results.Albums == nil || len(results.Albums.Albums) == 0 {
			continue
		}
		diag.sawResults = true

		for _, album := range results.Albums.Albums {
			if checked[album.ID] {
				continue
			}
			checked[album.ID] = true
			if !artistsMatch(album.Artists, record.Band) || !titlesMatch(album.Name, record.Recordname) {
				continue
			}
			diag.sawArtistMatch = true

			tracks, err := albumTracks(ctx, client, album.ID)
			if err != nil {
				return "", err
			}
			if match := bestTracklistMatch(tracks, track); match != nil {
				log.Printf(" using item from album tracklist: %s - %s (%s) [%s]", joinArtistNames(album.Artists), match.Name, album.Name, album.AlbumType)
				return match.ID, nil
			}
		}
	}
	return "", nil
}

// bestTracklistMatch prefers an exact (version-insensitive) title match over a fuzzy one.
func bestTracklistMatch(tracks []spotify.SimpleTrack, trackName string) *spotify.SimpleTrack {
	wanted := comparableTitle(trackName)
	for i := range tracks {
		if comparableTitle(tracks[i].Name) == wanted {
			return &tracks[i]
		}
	}
	for i := range tracks {
		if titlesMatch(tracks[i].Name, trackName) {
			return &tracks[i]
		}
	}
	return nil
}

var (
	albumTracksCacheMu sync.Mutex
	albumTracksCache   = make(map[spotify.ID][]spotify.SimpleTrack)
)

func albumTracks(ctx context.Context, client spotifySearcher, albumID spotify.ID) ([]spotify.SimpleTrack, error) {
	albumTracksCacheMu.Lock()
	cached, ok := albumTracksCache[albumID]
	albumTracksCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	var tracks []spotify.SimpleTrack
	for offset := 0; ; {
		page, err := client.GetAlbumTracks(ctx, albumID, spotify.Limit(spotifyAlbumTracksLimit), spotify.Offset(offset), spotify.Market(searchMarket))
		if err != nil {
			return nil, fmt.Errorf("get tracks of album %s: %w", albumID, err)
		}
		if page == nil {
			return nil, fmt.Errorf("get tracks of album %s: missing page", albumID)
		}
		tracks = append(tracks, page.Tracks...)
		if page.Next == "" {
			break
		}
		if len(page.Tracks) == 0 {
			return nil, fmt.Errorf("get tracks of album %s: empty page with a next page", albumID)
		}
		offset += len(page.Tracks)
	}

	albumTracksCacheMu.Lock()
	albumTracksCache[albumID] = tracks
	albumTracksCacheMu.Unlock()
	return tracks, nil
}

var (
	parentheticalPattern = regexp.MustCompile(`\s*[\(\[]([^\)\]]*)[\)\]]`)
	dashSuffixPattern    = regexp.MustCompile(`\s+[-–—]\s+([^-–—]+)$`)
	anyDashSuffixPattern = regexp.MustCompile(`\s+[-–—]\s+.*$`)
	// versionOnlyPattern matches annotations that denote the same recording, e.g. "Remastered 2011" or "Radio Edit".
	versionOnlyPattern = regexp.MustCompile(`(?i)^\s*(?:\d{4}\s+)?(?:(?:digital(?:ly)?\s+)?remaster(?:ed)?(?:\s+(?:version|\d{4}))?|radio\s+edit|single\s+edit|edit|(?:single|album|radio|original|lp)\s+version|mono(?:\s+version)?|stereo(?:\s+version)?|bonus(?:\s+track)?|explicit|clean|deluxe(?:\s+(?:edition|version))?)\s*$`)
	// featurePrefixPattern matches featured-artist annotations, e.g. "feat. Someone".
	featurePrefixPattern  = regexp.MustCompile(`(?i)^\s*(?:feat\.?|ft\.?|featuring|with)\s`)
	digitsPattern         = regexp.MustCompile(`\d+`)
	romanNumeralPattern   = regexp.MustCompile(`(?i)^(M{0,3}(CM|CD|D?C{0,3})(XC|XL|L?X{0,3})(IX|IV|V?I{0,3}))$`)
	artistCreditSeparator = regexp.MustCompile(`\s+(?:and|feat|ft|featuring|vs|x)\s+`)
)

func isVersionInfo(annotation string) bool {
	return versionOnlyPattern.MatchString(annotation) || featurePrefixPattern.MatchString(annotation)
}

// stripVersionInfo removes annotations that do not change the recording, such as
// "(feat. X)", "- Remastered 2011" or "(Radio Edit)". Other annotations like "(Live)" are kept.
func stripVersionInfo(s string) string {
	s = parentheticalPattern.ReplaceAllStringFunc(s, func(match string) string {
		inner := parentheticalPattern.FindStringSubmatch(match)[1]
		if isVersionInfo(inner) {
			return ""
		}
		return match
	})
	if loc := dashSuffixPattern.FindStringSubmatchIndex(s); loc != nil && isVersionInfo(s[loc[2]:loc[3]]) {
		s = s[:loc[0]]
	}
	return strings.TrimSpace(inlineFeaturePattern.ReplaceAllString(s, ""))
}

// comparableTitle normalizes a title for comparison, ignoring version annotations.
func comparableTitle(s string) string {
	return normalizeForComparison(stripVersionInfo(s))
}

// baseTitle normalizes a title after removing all parenthetical and dash-suffixed annotations.
func baseTitle(s string) string {
	s = parentheticalPattern.ReplaceAllString(s, "")
	s = anyDashSuffixPattern.ReplaceAllString(s, "")
	return normalizeForComparison(s)
}

// titlesMatch reports whether two track or album titles refer to the same work.
func titlesMatch(a, b string) bool {
	ca, cb := comparableTitle(a), comparableTitle(b)
	if ca == "" || cb == "" {
		return false
	}
	ca, cb = normalizeRomanNumeralPairs(ca, cb)
	if ca == cb {
		return true
	}

	if strings.Join(digitsPattern.FindAllString(ca, -1), " ") != strings.Join(digitsPattern.FindAllString(cb, -1), " ") {
		return false
	}

	// Generic annotations may be omitted, but numbered parts must remain distinct.
	ba, bb := baseTitle(a), baseTitle(b)
	if ba != "" && ba == bb && (ca == ba || cb == bb) {
		return true
	}

	return similarity(ca, cb) >= similarityThreshold
}

func normalizeRomanNumeralPairs(a, b string) (string, string) {
	aWords, bWords := strings.Fields(a), strings.Fields(b)
	for i := 0; i < len(aWords) && i < len(bWords); i++ {
		_, aErr := strconv.Atoi(aWords[i])
		_, bErr := strconv.Atoi(bWords[i])
		aIsNumber, bIsNumber := aErr == nil, bErr == nil
		aRoman, aIsRoman := romanNumeralValue(aWords[i])
		bRoman, bIsRoman := romanNumeralValue(bWords[i])

		if (aIsNumber && bIsRoman) || (aIsRoman && bIsNumber) || (aIsRoman && bIsRoman) {
			if aIsRoman {
				aWords[i] = strconv.Itoa(aRoman)
			}
			if bIsRoman {
				bWords[i] = strconv.Itoa(bRoman)
			}
		}
	}
	return strings.Join(aWords, " "), strings.Join(bWords, " ")
}

func romanNumeralValue(s string) (int, bool) {
	if s == "" || !romanNumeralPattern.MatchString(s) {
		return 0, false
	}

	values := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	value, previous := 0, 0
	for _, numeral := range strings.ToUpper(s) {
		current := values[numeral]
		if current < previous {
			value -= current
		} else {
			value += current
		}
		previous = current
	}
	return value, true
}

func comparableArtist(s string) string {
	return strings.TrimPrefix(normalizeForComparison(s), "the ")
}

// artistsMatch reports whether Spotify's artist credits match the Plattentests band name.
// Any single artist, all artists combined, or the primary artist of a collaboration may match.
func artistsMatch(artists []spotify.SimpleArtist, band string) bool {
	target := comparableArtist(band)
	if target == "" {
		return false
	}

	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		if name := comparableArtist(artist.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return false
	}

	candidates := append(append([]string{}, names...), strings.Join(names, " "), strings.Join(names, " and "))
	for _, candidate := range candidates {
		if similarity(candidate, target) >= similarityThreshold {
			return true
		}
	}

	parts := artistCreditSeparator.Split(target, -1)
	if len(parts) > 1 {
		primary := strings.TrimPrefix(parts[0], "the ")
		return similarity(names[0], primary) >= similarityThreshold
	}
	return false
}

// similarity returns 1 minus the Levenshtein distance relative to the longer string.
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	longest := maxInt(utf8.RuneCountInString(a), utf8.RuneCountInString(b))
	if longest == 0 {
		return 1
	}
	return 1 - float64(levenshtein.ComputeDistance(a, b))/float64(longest)
}

func joinArtistNames(artists []spotify.SimpleArtist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		names = append(names, artist.Name)
	}
	return strings.Join(names, ", ")
}

func logTrackResults(tracks []spotify.FullTrack) {
	for i, item := range tracks {
		if i >= MaxSearchResults {
			break
		}
		log.Printf(" found item: %s - %s  (%s) [%s]", joinArtistNames(item.Artists), item.Name, item.Album.Name, item.Album.AlbumType)
	}
}

// logMiss records everything needed to replay a failed lookup.
func logMiss(record crawler.Record, trackName string, outcome searchOutcome) {
	log.Printf("MISS band=%q track=%q record=%q year=%q reason=%q queries=%q",
		record.Band, trackName, record.Recordname, record.ReleaseYear, outcome.Reason, outcome.Queries)
}

func appendUnique(list []string, items ...string) []string {
	for _, item := range items {
		exists := false
		for _, existing := range list {
			if existing == item {
				exists = true
				break
			}
		}
		if !exists {
			list = append(list, item)
		}
	}
	return list
}
