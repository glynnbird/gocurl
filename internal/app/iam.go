package gcurl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TokenCache represents the structure of the cached token file
type TokenCache struct {
	APIKey       string    `json:"api_key"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// IAMTokenResponse represents the response from IBM IAM token service
type IAMTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Expiration   int64  `json:"expiration"`
}

const (
	iamTokenURL = "https://iam.cloud.ibm.com/identity/token"
	cacheFile   = ".gocurl.json"
)

// GetIAMToken retrieves a valid IAM bearer token, using cache if available
func GetIAMToken(apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("IAM API key is empty")
	}

	// Try to load from cache first
	cached, err := loadTokenCache(apiKey)
	if err == nil && cached != nil {
		// Check if token is still valid (with 5 minute buffer)
		if time.Now().Add(5 * time.Minute).Before(cached.ExpiresAt) {
			return cached.AccessToken, nil
		}
	}

	// Token not in cache or expired, fetch new one
	token, err := fetchIAMToken(apiKey)
	if err != nil {
		return "", err
	}

	// Cache the new token
	err = saveTokenCache(apiKey, token)
	if err != nil {
		// Log warning but don't fail - we have the token
		fmt.Fprintf(os.Stderr, "Warning: failed to cache token: %v\n", err)
	}

	return token.AccessToken, nil
}

// fetchIAMToken requests a new token from IBM IAM service
func fetchIAMToken(apiKey string) (*IAMTokenResponse, error) {
	// Prepare request body
	data := fmt.Sprintf("grant_type=urn:ibm:params:oauth:grant-type:apikey&apikey=%s", apiKey)

	req, err := http.NewRequest("POST", iamTokenURL, bytes.NewBufferString(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IAM token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read IAM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IAM token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp IAMTokenResponse
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IAM token response: %w", err)
	}

	return &tokenResp, nil
}

// loadTokenCache loads the cached token from ~/.gocurl.json
func loadTokenCache(apiKey string) (*TokenCache, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cache file does not exist")
		}
		return nil, err
	}

	var cache TokenCache
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Verify the cached token is for the same API key
	if cache.APIKey != apiKey {
		return nil, fmt.Errorf("cached token is for different API key")
	}

	return &cache, nil
}

// saveTokenCache saves the token to ~/.gocurl.json
func saveTokenCache(apiKey string, token *IAMTokenResponse) error {
	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	cache := TokenCache{
		APIKey:       apiKey,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    token.TokenType,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	// Write with restricted permissions (0600 - owner read/write only)
	err = os.WriteFile(cachePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// getCachePath returns the full path to the cache file
func getCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, cacheFile), nil
}
