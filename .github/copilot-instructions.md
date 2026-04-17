# GitHub Copilot Instructions

Go application that crawls music album reviews from [Plattentests.de](https://www.plattentests.de) and creates Spotify playlists from the weekly highlights.

## Build & Run

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./cmd/creator/...

# Run a single test by name
go test ./cmd/creator/... -run TestSanitizeTrackname

# Build the web UI
cd webui && go build ./...

# Run the web UI locally (port 8081, requires .env)
make web

# Build Docker image
make docker-web-build
```

## Architecture

The web UI (`webui/main.go`) is the only `main` package and the sole entry point. It imports and orchestrates:

- `cmd/crawler` — library package (not `main`) that scrapes Plattentests.de for weekly album reviews and highlight tracks using goquery. Fetches individual record pages concurrently with goroutines.
- `cmd/creator` — library package (not `main`) that searches Spotify for crawled tracks and builds a playlist. Uses a scoring system to select the best match: album type (album > single > EP) and track/record name matching.
- `internal/auth` — handles Spotify OAuth2 token lifecycle, persisting tokens as JSON in Azure Blob Storage.
- `cmd/token` — standalone CLI (`main` package) for initial token generation; not imported by the web UI.

Data flows: `crawler.GetRecordsOfTheWeek()` → `creator.CreatePlaylist(playlistID)` → Spotify API.

The `Record` and `Track` types are defined in `cmd/crawler` and re-used by `cmd/creator` and `webui`.

## Key Conventions

- **Character encoding**: Plattentests.de serves ISO-8859-1. Always decode responses using `charset.NewReader()` (see `crawler.newDocumentFromPlattentestsResponse`). Never use `ioutil.ReadAll` directly on Plattentests responses.
- **Fuzzy matching**: Artist and track names are compared between Plattentests and Spotify using Levenshtein distance with a 0.8 similarity threshold. Use `normalizeForComparison()` before comparing strings — it lowercases, removes accents/diacritics, and strips punctuation.
- **Search sanitization**: Track names are cleaned via `sanitizeTrackname()` before Spotify API queries — removes feat/with annotations, quotes, brackets, accents, and special punctuation.
- **Config injection**: Environment variables are loaded via `github.com/kelseyhightower/envconfig` into per-package `config` structs. Add new config fields to the relevant struct with `envconfig` tags.
- **Testing pattern**: All tests use table-driven style with `t.Run()` subtests. Crawler tests use `httptest.NewServer` with mock HTML. See `cmd/creator/sanitize_test.go` and `cmd/crawler/crawler_test.go` for reference.
- **Web UI auth**: The `/createPlaylist` endpoint uses HTTP Basic Auth checked against `CREATOR_USER` and `CREATOR_PASSWORD` env vars.
- **Templates**: Gin serves HTML templates from `webui/templates/` with static assets from `webui/assets/`. Templates are parsed with `template.ParseFiles`, not Gin's built-in template loading.
- **Deployment**: Docker image built from `webui/Dockerfile`, deployed to Azure Container Apps via GitHub Actions (`deploy-aca.yml`).
