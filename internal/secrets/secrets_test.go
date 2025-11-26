package secrets

import (
	"os"
	"testing"
)

func TestGetSecretWithFallback(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		envVarName     string
		envValue       string
		setupEnv       bool
		setupKeyVault  bool
		expectedResult string
	}{
		{
			name:           "fallback to environment variable when Key Vault URL not set",
			secretName:     "TEST-SECRET",
			envVarName:     "TEST_SECRET",
			envValue:       "test-value-from-env",
			setupEnv:       true,
			setupKeyVault:  false,
			expectedResult: "test-value-from-env",
		},
		{
			name:           "return empty string when neither Key Vault nor env var set",
			secretName:     "NONEXISTENT-SECRET",
			envVarName:     "NONEXISTENT_SECRET",
			setupEnv:       false,
			setupKeyVault:  false,
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			os.Unsetenv("AZURE_KEYVAULT_URL")
			os.Unsetenv(tt.envVarName)

			// Setup test environment
			if tt.setupEnv {
				os.Setenv(tt.envVarName, tt.envValue)
				defer os.Unsetenv(tt.envVarName)
			}

			// Test GetSecretWithFallback
			result := GetSecretWithFallback(tt.secretName, tt.envVarName)
			if result != tt.expectedResult {
				t.Errorf("GetSecretWithFallback(%q, %q) = %q, want %q", tt.secretName, tt.envVarName, result, tt.expectedResult)
			}
		})
	}
}

func TestGetSecretWithFallbackKeyVaultURL(t *testing.T) {
	// Test that when AZURE_KEYVAULT_URL is set but invalid, it falls back to env var
	os.Setenv("AZURE_KEYVAULT_URL", "https://invalid-keyvault.vault.azure.net/")
	os.Setenv("TEST_ENV_VAR", "fallback-value")
	defer os.Unsetenv("AZURE_KEYVAULT_URL")
	defer os.Unsetenv("TEST_ENV_VAR")

	result := GetSecretWithFallback("TEST-SECRET", "TEST_ENV_VAR")
	if result != "fallback-value" {
		t.Errorf("GetSecretWithFallback should fall back to env var when Key Vault fails, got %q", result)
	}
}
