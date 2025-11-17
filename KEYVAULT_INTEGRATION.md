# Azure Key Vault Integration

This document describes how the application has been updated to support Azure Key Vault for secret management.

## Overview

The application now supports two methods for managing secrets:

1. **Environment Variables** (backward compatible): Secrets are read from environment variables or `.env` file
2. **Azure Key Vault** (recommended for production): Secrets are securely stored in Azure Key Vault

## How It Works

The new `internal/secrets` package provides a unified interface for retrieving secrets. When `AZURE_KEYVAULT_URL` is set, the package attempts to retrieve secrets from Key Vault first, then falls back to environment variables if Key Vault is unavailable or the secret is not found.

### Secret Naming Convention

Azure Key Vault does not allow underscores in secret names, so we use hyphens instead:

| Environment Variable | Key Vault Secret Name |
|---------------------|----------------------|
| `SPOTIFY_ID` | `SPOTIFY-ID` |
| `SPOTIFY_SECRET` | `SPOTIFY-SECRET` |
| `PLAYLIST_ID` | `PLAYLIST-ID` |
| `PLAYLIST_ID_PROD` | `PLAYLIST-ID-PROD` |
| `AZ_ACCOUNT` | `AZ-ACCOUNT` |
| `AZ_KEY` | `AZ-KEY` |
| `AZ_CONTAINER` | `AZ-CONTAINER` |
| `CREATOR_USER` | `CREATOR-USER` |
| `CREATOR_PASSWORD` | `CREATOR-PASSWORD` |

## Configuration

### Using Environment Variables (Default Behavior)

If `AZURE_KEYVAULT_URL` is not set, the application will use environment variables as before. No changes are required to existing deployments.

### Using Azure Key Vault

1. Create an Azure Key Vault or use an existing one
2. Add the secrets to your Key Vault with the names from the table above
3. Set the `AZURE_KEYVAULT_URL` environment variable to your Key Vault URL:
   ```
   AZURE_KEYVAULT_URL=https://your-keyvault.vault.azure.net/
   ```
4. Ensure your application has access to the Key Vault using one of these methods:
   - **Managed Identity** (recommended for Azure services)
   - **Service Principal** with `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, and `AZURE_TENANT_ID`
   - **Azure CLI** authentication (for local development)

## Authentication

The application uses `DefaultAzureCredential` from the Azure SDK, which automatically tries multiple authentication methods in this order:

1. Environment variables (`AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID`)
2. Managed Identity (when running in Azure)
3. Azure CLI credentials (for local development)
4. Interactive browser authentication

## Code Changes

### Modified Files

1. **`internal/secrets/secrets.go`** (new): Core secrets management package
   - `GetSecret()`: Retrieves a secret from Key Vault
   - `GetSecretWithFallback()`: Retrieves from Key Vault with env var fallback
   - Includes caching to minimize Key Vault calls

2. **`internal/auth/auth.go`**: Updated to use secrets package for Azure Storage credentials
   - Removed `AzAccountName`, `AzAccountKey`, `AzContainerName` from config struct
   - Updated `GetAccountInfo()` to use `secrets.GetSecretWithFallback()`

3. **`cmd/creator/main.go`**: Updated to use secrets package for Spotify and playlist credentials
   - Removed Azure Storage fields from config struct
   - Updated to retrieve Spotify and playlist IDs from secrets package

4. **`webui/main.go`**: Updated to use secrets package for authentication
   - Updated `checkAuth()` to retrieve credentials from secrets package
   - Updated playlist ID retrieval to use secrets package

5. **`cmd/token/main.go`**: Updated to use auth package changes
   - Removed Azure Storage fields from config struct

### Dependencies Added

- `github.com/Azure/azure-sdk-for-go/sdk/azidentity` - Azure authentication
- `github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets` - Key Vault client

## Testing

Run the tests to verify functionality:

```bash
# Test the secrets package
go test ./internal/secrets -v

# Test the creator package (uses secrets indirectly)
go test ./cmd/creator -v

# Build all packages
go build ./...
```

## Migration Guide

### For Local Development

No changes required. Continue using your `.env` file with the existing environment variable names.

### For Production Deployment

1. Create secrets in Azure Key Vault with the hyphenated names (e.g., `SPOTIFY-ID`)
2. Grant your application access to the Key Vault
3. Add `AZURE_KEYVAULT_URL` to your application configuration
4. Remove sensitive secrets from environment variables (they will now come from Key Vault)
5. Keep non-sensitive configuration like `TOKEN_FILE` in environment variables

## Backward Compatibility

The implementation is fully backward compatible:
- If `AZURE_KEYVAULT_URL` is not set, environment variables are used
- If Key Vault access fails, the application falls back to environment variables
- No changes are required to existing deployments

## Security Benefits

1. **Centralized Secret Management**: All secrets in one secure location
2. **Access Control**: Fine-grained access control via Azure RBAC
3. **Audit Logging**: Key Vault logs all secret access attempts
4. **Automatic Rotation**: Supports secret rotation without application changes
5. **No Secrets in Code**: Secrets are never stored in source code or configuration files
