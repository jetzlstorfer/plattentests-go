# Agent Instructions

## Project Summary

This Go 1.26 application crawls weekly highlights from [Plattentests.de](https://www.plattentests.de), finds the highlighted tracks on Spotify, and builds playlists. A Gin web UI displays current records and search results and can trigger playlist creation.

## Repository Layout

```text
cmd/crawler/       Plattentests scraper and shared Record/Track types
cmd/creator/       Spotify matching and playlist creation
cmd/token/         Standalone CLI for generating and storing the Spotify token
internal/auth/     Spotify OAuth2 and Azure Blob token persistence
webui/             Gin server, templates, and static assets
docs/              Operational documentation, including Easy Auth setup
.github/workflows/ CI, security scanning, image build, and ACA deployment
```

`webui/main.go` is the web executable. `cmd/token/main.go` is a separate CLI executable. The crawler and creator directories are library packages despite living under `cmd/`.

## Build and Test

```bash
go test ./...                         # all tests
go test ./cmd/creator/...             # one package
go test ./cmd/creator/... -run TestSanitizeTrackname
cd webui && go build ./...            # build the web executable
make run                              # run on :8081; requires .env
make token                            # generate and upload a Spotify token
make lint                             # requires golangci-lint
make docker-web-build                 # build the production image
make docker-web-run                   # run the image on :8081
```

Run `gofmt` on changed Go files and prefer the narrowest relevant test before running `go test ./...`.

## Configuration

Copy `env.sample` to `.env`. The Makefile includes `.env` and exports its values. Required runtime variables are declared with `envconfig` in the package that consumes them:

- `SPOTIFY_ID`, `SPOTIFY_SECRET`
- `PLAYLIST_ID` and `PLAYLIST_ID_PROD`
- `AZ_ACCOUNT`, `AZ_KEY`, `AZ_CONTAINER`
- `TOKEN_FILE`

Do not commit `.env`, Spotify credentials, Azure keys, or OAuth tokens. Active web and creator paths read `PLAYLIST_ID_PROD`; the unused `PROD_PLAYLIST_ID` field in the creator config is legacy code and should not be treated as the runtime contract.

## Important Conventions

- Plattentests responses are ISO-8859-1. Decode them through `newDocumentFromPlattentestsResponse`/`charset.NewReader`; do not parse the raw response body as UTF-8.
- Prefer `crawler.GetRecordsOfTheWeekSafe()` in code that can report errors. `GetRecordsOfTheWeek()` is the compatibility wrapper that discards errors.
- Normalize Spotify comparisons with `normalizeForComparison()` and sanitize search queries with `sanitizeTrackname()`.
- Preserve the existing concurrent crawler and creator patterns: bounded work is coordinated with `sync.WaitGroup`, and result ordering matters.
- Tests use table-driven `t.Run` cases. Crawler HTTP tests use `httptest.NewServer`; web handlers are tested with `httptest` and dependency seams in `webui/main_test.go`.
- Templates are parsed explicitly with `template.ParseFiles`. When adding a page, update the parse list and route together. Static files are served from `webui/assets/`.

## Web Authentication and Deployment

The playlist creation flow is protected in production by Azure Container Apps Easy Auth. Azure injects `X-MS-CLIENT-PRINCIPAL-NAME`; the application redirects missing production identities to `/.auth/login/aad`. Local requests are allowed without the header. Keep application behavior and `docs/easy-auth-setup.md` aligned.

`webui/Dockerfile` builds the production image. GitHub Actions runs lint, CodeQL, and dependency review; `deploy-aca.yml` builds and pushes the image, updates the Azure Container App, and configures Easy Auth. Do not edit generated deployment state or add credentials to workflow files.
