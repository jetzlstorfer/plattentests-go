package secrets

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

var (
	client     *azsecrets.Client
	clientOnce sync.Once
	cache      = make(map[string]string)
	cacheMutex sync.RWMutex
)

// GetClient returns a singleton Azure Key Vault client
func GetClient() (*azsecrets.Client, error) {
	var err error
	clientOnce.Do(func() {
		keyVaultURL := os.Getenv("AZURE_KEYVAULT_URL")
		if keyVaultURL == "" {
			err = fmt.Errorf("AZURE_KEYVAULT_URL environment variable not set")
			return
		}

		cred, credErr := azidentity.NewDefaultAzureCredential(nil)
		if credErr != nil {
			err = fmt.Errorf("failed to create Azure credential: %w", credErr)
			return
		}

		client, err = azsecrets.NewClient(keyVaultURL, cred, nil)
		if err != nil {
			err = fmt.Errorf("failed to create Key Vault client: %w", err)
		}
	})
	return client, err
}

// GetSecret retrieves a secret from Azure Key Vault with caching
func GetSecret(secretName string) (string, error) {
	// Check cache first
	cacheMutex.RLock()
	if val, ok := cache[secretName]; ok {
		cacheMutex.RUnlock()
		return val, nil
	}
	cacheMutex.RUnlock()

	// Get client
	client, err := GetClient()
	if err != nil {
		return "", err
	}

	// Retrieve secret from Key Vault
	resp, err := client.GetSecret(context.Background(), secretName, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	if resp.Value == nil {
		return "", fmt.Errorf("secret %s has no value", secretName)
	}

	secretValue := *resp.Value

	// Cache the secret
	cacheMutex.Lock()
	cache[secretName] = secretValue
	cacheMutex.Unlock()

	return secretValue, nil
}

// MustGetSecret retrieves a secret from Azure Key Vault and panics if it fails
func MustGetSecret(secretName string) string {
	val, err := GetSecret(secretName)
	if err != nil {
		log.Fatalf("Failed to get secret %s: %v", secretName, err)
	}
	return val
}

// GetSecretWithFallback tries to get a secret from Key Vault, falling back to environment variable
func GetSecretWithFallback(secretName, envVarName string) string {
	// Try Key Vault first if configured
	if os.Getenv("AZURE_KEYVAULT_URL") != "" {
		val, err := GetSecret(secretName)
		if err == nil {
			return val
		}
		log.Printf("Warning: failed to get secret %s from Key Vault: %v, falling back to environment variable", secretName, envVarName)
	}

	// Fallback to environment variable
	envVal := os.Getenv(envVarName)
	if envVal == "" {
		log.Printf("Warning: neither Key Vault secret %s nor environment variable %s is set", secretName, envVarName)
	}
	return envVal
}
