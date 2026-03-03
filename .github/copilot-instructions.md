# GitHub Copilot Instructions

This is a Go-based application that crawls music album reviews from [Plattentests.de](https://www.plattentests.de) and automatically creates Spotify playlists from the weekly highlights. Please follow these guidelines when contributing:

## Project Overview

The application consists of:
- A **web crawler** that fetches album reviews and highlight tracks from Plattentests.de
- A **Spotify playlist creator** that searches for and adds highlight tracks to a Spotify playlist
- A **web UI** (Gin-based) that displays the current weekly highlights and allows playlist creation
- An **authentication module** that manages Spotify OAuth2 tokens stored in Azure Blob Storage

## Repository Structure

- `cmd/crawler/` — Web crawler for fetching album reviews and highlight tracks from Plattentests.de
- `cmd/creator/` — Playlist creation logic; searches Spotify and builds playlists from crawled data
- `cmd/token/` — CLI tool for generating and uploading a Spotify OAuth2 token to Azure Blob Storage
- `internal/auth/` — Spotify OAuth2 authentication and Azure Blob Storage token management
- `webui/` — Gin web server with HTML templates and static assets; runs on port 8081
- `env` — Template for required environment variables (copy to `.env` and fill in values)
- `Makefile` — Build and run targets

## Development Setup

1. Copy `env` to `.env` and fill in all required values:
   - `SPOTIFY_ID` and `SPOTIFY_SECRET` — Spotify API credentials
   - `PLAYLIST_ID` and `PLAYLIST_ID_PROD` — Target Spotify playlist IDs
   - `AZ_ACCOUNT`, `AZ_KEY`, `AZ_CONTAINER` — Azure Blob Storage credentials for token persistence
   - `TOKEN_FILE` — Filename of the token blob (default: `token.txt`)

2. Use the devcontainer (`.devcontainer/`) or GitHub Codespaces for a pre-configured environment.

## Build & Run

```bash
# Run the web UI (port 8081)
make web

# Generate and store a Spotify OAuth2 token in Azure
make token

# Build and push Docker image for the web UI
make docker-web-build

# Build Go packages directly
cd webui && go build ./...
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./cmd/creator/...
```

Tests use the standard Go `testing` package with table-driven test patterns. Unit tests live alongside the code they test (e.g., `cmd/creator/sanitize_test.go`).

## Code Standards

- Follow standard Go conventions and idiomatic patterns
- Use table-driven tests when writing unit tests
- Keep packages focused: crawler logic in `cmd/crawler`, Spotify logic in `cmd/creator`, auth in `internal/auth`
- Character encoding: the source website uses ISO-8859-1; always decode to UTF-8 using `charmap.ISO8859_1.NewDecoder()`
- String matching uses Levenshtein distance (threshold 0.8) to handle minor spelling differences between Plattentests.de and Spotify

## Key Guidelines

1. The `cmd/crawler` and `cmd/creator` packages are not `main` packages — they export functions used by `webui/main.go` and other entry points
2. Spotify search uses a scoring system: album type (album > single > EP) and track/record name matching affect which result is selected
3. Environment variables are loaded via `github.com/kelseyhightower/envconfig` — add new config fields to the `config` struct in the relevant package
4. The web UI runs on port 8081; it requires all environment variables to be set (including Azure and Spotify credentials) to create playlists
5. Docker builds use `webui/Dockerfile` and are deployed to Azure Container Apps via GitHub Actions
