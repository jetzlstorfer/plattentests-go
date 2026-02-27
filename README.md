# Plattentests.de - Highlights der Woche

[![Build and deploy to Azure Container Apps](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-aca.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-aca.yml)
[![CodeQL](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/codeql.yml)
[![Dependency Review](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/dependency-review.yml)


# Structure

This project is a Go-based application for crawling and displaying music album reviews from Plattentests.de. It consists of multiple components:

## Requirements

- **Go 1.25.6** or later
- Environment variables configured in `.env` file (see `env` template)

## Project Layout

```
plattentests-go/
├── cmd/                    # Command-line applications
│   ├── crawler/           # Web crawler for fetching album reviews
│   │   └── main.go
│   ├── creator/           # Playlist creation functionality
│   │   ├── main.go
│   │   └── sanitize_test.go
│   └── token/             # Authentication token management
│       └── main.go
├── internal/              # Private application code
│   └── auth/             # Authentication logic
│       └── auth.go
├── webui/                # Web frontend
│   ├── main.go           # Web server
│   ├── Dockerfile        # Container image for web UI
│   ├── assets/           # Static web assets
│   │   ├── css/          # Stylesheets
│   │   │   ├── modern.css
│   │   │   └── style.css
│   │   └── js/           # JavaScript files
│   │       └── func.js   # Frontend functions for UI interactions
│   └── templates/        # HTML templates
│       ├── createPlaylist.tmpl
│       ├── records.tmpl
│       └── utils.tmpl
├── go.mod                # Go module definition
├── Makefile             # Build automation
├── LICENSE              # Project license
└── env                  # Environment configuration template
```

## Components

- **Crawler** (`cmd/crawler`): Fetches album reviews and data from Plattentests.de
- **Creator** (`cmd/creator`): Creates playlists based on crawled data with sanitization features
- **Token Manager** (`cmd/token`): Handles authentication tokens for external services
- **Web UI** (`webui`): Modern web interface for browsing and interacting with album data
- **Auth** (`internal/auth`): Internal authentication and authorization logic



# Usage


💡 For your own convenience, make use of Codespaces or run it locally as devcontainer.

There is a `Makefile` with multiple targets to be used. 
⚠️ Make sure you have the proper `ENV` variables set in a `.env` file.

- To create a token and store it in Azure:
    ```
    make token
    ```

- To run the project locally as Go binary:
    ```
    make run
    ```

- To run the project locally as a function:
    ```
    make run-function
    ```

- To run the web-frontend of the project (located in `./webui`):
    ```
    make web
    ```



