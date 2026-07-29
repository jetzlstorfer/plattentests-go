# GitHub Copilot Instructions

This repository is a Go 1.25 application that crawls weekly album highlights from [Plattentests.de](https://www.plattentests.de), matches their highlighted tracks on Spotify, and creates playlists. The Gin web UI displays records, supports search, and runs playlist creation.

## Commands

```bash
go test ./...                         # all tests
go test ./cmd/crawler/...             # crawler tests
go test ./cmd/creator/... -run TestSanitizeTrackname
cd webui && go build ./...            # web executable
make run                              # local server on :8081; requires .env
make token                            # generate/store Spotify OAuth token
make lint                             # golangci-lint
make docker-web-build                 # production image
make docker-web-run                   # image on localhost:8081
```

Use `gofmt` for Go changes. Start with the narrowest relevant test, then run `go test ./...` for changes that cross package boundaries.

## Architecture and Data Flow

- `cmd/crawler` is a library package. It scrapes Plattentests with goquery, fetches record details concurrently, and owns the shared `Record` and `Track` types.
- `cmd/creator` is a library package. It searches Spotify, scores candidate albums/tracks, and adds matched tracks to the selected playlist.
- `internal/auth` manages Spotify OAuth2 clients and persists token JSON in Azure Blob Storage.
- `webui/main.go` is the Gin web executable and orchestrates crawler and creator operations. It renders records, search, playlist, and playlist-result pages.
- `cmd/token/main.go` is a separate executable used to obtain and upload the initial Spotify token.

The principal playlist flow is:

```text
crawler.GetRecordsOfTheWeekSafe()
	-> creator.CreatePlaylist(playlistID)
	-> Spotify search/matching
	-> Spotify playlist update
```

## Crawler and Text Handling

- Plattentests serves ISO-8859-1. Always build documents through `newDocumentFromPlattentestsResponse`, which uses `charset.NewReader`. Do not read and parse Plattentests response bodies as UTF-8.
- Use `GetRecordsOfTheWeekSafe()` when the caller can surface HTTP or parse errors. `GetRecordsOfTheWeek()` is a compatibility wrapper that returns an empty result on failure.
- Preserve deterministic result ordering around concurrent fetches. Existing crawler and creator fan-out uses goroutines plus `sync.WaitGroup`.
- Crawler tests use `httptest.NewServer` and mock HTML. Keep network-dependent logic behind testable HTTP boundaries.

## Spotify Matching

- Call `sanitizeTrackname()` before constructing Spotify searches. It removes feature annotations, brackets, quotes, accents, and punctuation that degrade search results.
- Call `normalizeForComparison()` before comparing artists, tracks, or albums. It lowercases, removes diacritics and punctuation, and supports the existing Levenshtein similarity checks.
- Keep scoring behavior explicit: album releases rank above singles and EPs, then normalized artist, track, and record names determine the best match.
- Do not silently drop not-found tracks. Creator results feed the web UI's run summary and production-playlist controls.

## Configuration

Configuration is loaded with `github.com/kelseyhightower/envconfig` into the package-local config struct. Add variables to the consuming package rather than introducing global configuration.

Copy `env.sample` to `.env`; the Makefile includes and exports it. Runtime variables are:

- `SPOTIFY_ID`, `SPOTIFY_SECRET`
- `PLAYLIST_ID`, `PLAYLIST_ID_PROD`
- `AZ_ACCOUNT`, `AZ_KEY`, `AZ_CONTAINER`
- `TOKEN_FILE`

Active web and creator paths read `PLAYLIST_ID_PROD`, matching `env.sample`. The unused `PROD_PLAYLIST_ID` field in the creator config is legacy code; do not use it as the runtime contract. Never commit `.env`, tokens, client secrets, Azure keys, or populated config values.

## Web UI

- Templates live in `webui/templates/` and are parsed explicitly with `template.ParseFiles`; Gin's glob-based template loader is not used. Add new templates to the parse list and register their route in the same change.
- Static assets are served from `webui/assets/`. Preserve the existing template and CSS structure instead of introducing an unrelated frontend framework.
- Handler dependencies are exposed through package-level function variables in `webui/main.go` so `webui/main_test.go` can replace crawler and creator calls. Restore any replacement with `t.Cleanup` or `defer`.
- User-facing creator failures go through `friendlyCreatePlaylistError`; avoid rendering raw Spotify, Azure, or internal errors.
- Keep the create-playlist run summary and not-found track integration intact; tests in `webui/main_test.go` cover these page-level contracts.

## Easy Auth

Azure Container Apps Easy Auth protects playlist creation in production. The platform injects `X-MS-CLIENT-PRINCIPAL-NAME` for an authenticated user. In the application:

- production requests without a principal redirect to `/.auth/login/aad?post_login_redirect_uri=...`;
- local requests are allowed without the header for development;
- authenticated requests render the create-playlist page and may invoke the creator.

Keep this behavior, its tests, and `docs/easy-auth-setup.md` synchronized. Do not replace platform authentication with application-managed credentials.

## Testing and Delivery

- Use table-driven tests with `t.Run` for input matrices. See `cmd/creator/sanitize_test.go` and crawler tests for established patterns.
- Web handler tests use `net/http/httptest`; assert status, redirect location, and rendered body where relevant.
- `webui/Dockerfile` is a multi-stage build that compiles the web executable and copies templates/assets into the runtime image.
- `.github/workflows/lint.yml` runs golangci-lint. CodeQL and dependency review run in dedicated workflows.
- `.github/workflows/deploy-aca.yml` builds and pushes the Docker image, updates the Azure Container App, and configures Easy Auth.
