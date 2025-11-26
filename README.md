# Plattentests.de - Highlights der Woche

[![Build and deploy to Azure Container Apps](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-aca.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-aca.yml)
[![CodeQL](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/codeql.yml)
[![Dependency Review](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/dependency-review.yml)

👩‍💻 **Please note that this project currently serves multiple purposes** 👨‍💻

1. The original purpose of generating a [Spotify playlist](https://open.spotify.com/playlist/2Bc5TRdMTj6OHwt32x5T6Y?si=c7cf976d4d124bef) that lists all "highlights" of the week of my personal favourite music website [Plattentests.de](https://plattentests.de).
1. The purpose of getting to know more about serverless, Azure functions, and Azure Container Apps
1. A playground for features like
   - Codespaces & devcontainers,
   - GitHub actions,
   - GitHub Copilot and other features of GitHub.

Therefore, some commit messages might not be useful at the moment :)

# Structure

In the root directory, you will find the following folders:
- `cmd`: Contains the main function of the project for the Azure Function
- `webui`: Contains the web frontend of the project --> this is an Azure Container App with its own Dockerfile


# Usage


💡 For your own convenience, make use of Codespaces or run it locally as devcontainer.

There is a `Makefile` with multiple targets to be used. 
⚠️ Make sure you have the proper `ENV` variables set in a `.env` file.

## Configuration

### Environment Variables vs Azure Key Vault

This application supports two methods for managing secrets:

1. **Environment Variables** (traditional method): Set secrets in a `.env` file or as environment variables
2. **Azure Key Vault** (recommended for production): Store secrets securely in Azure Key Vault

#### Using Azure Key Vault

To use Azure Key Vault for secret management:

1. Set the `AZURE_KEYVAULT_URL` environment variable to your Key Vault URL (e.g., `https://your-keyvault.vault.azure.net/`)
2. Store the following secrets in your Key Vault with these names:
   - `SPOTIFY-ID` - Spotify API Client ID
   - `SPOTIFY-SECRET` - Spotify API Client Secret
   - `PLAYLIST-ID` - Default Spotify Playlist ID
   - `PLAYLIST-ID-PROD` - Production Spotify Playlist ID
   - `AZ-ACCOUNT` - Azure Storage Account Name
   - `AZ-KEY` - Azure Storage Account Key
   - `AZ-CONTAINER` - Azure Storage Container Name
   - `CREATOR-USER` - Basic auth username for playlist creation
   - `CREATOR-PASSWORD` - Basic auth password for playlist creation

3. Ensure your application has access to the Key Vault using Azure managed identity or other authentication methods supported by `DefaultAzureCredential`

**Note:** If `AZURE_KEYVAULT_URL` is not set, the application will fall back to using environment variables with the original names (e.g., `SPOTIFY_ID`, `SPOTIFY_SECRET`, etc.).

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


## As Docker container

You can also run the project as a Docker container.

- Azure Function: 
    ```
    docker build -t plattentests-go .
    docker run -p 8080:8080 plattentests-go
    ```
- Web Frontend (make sure it points to the correct function URL)
    ```
    cd webui
    docker build -t plattentests-go-web .
    docker run -p 8081:8081 plattentests-go-web
    ```


