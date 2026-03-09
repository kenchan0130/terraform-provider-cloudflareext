package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CloudflareClient holds the configuration for Cloudflare API calls.
type CloudflareClient struct {
	HTTPClient *http.Client
	BaseURL    string
	APIToken   string
	AccountID  string
}

// CloudflareResponse represents the standard Cloudflare API response envelope.
type CloudflareResponse[T any] struct {
	Success bool              `json:"success"`
	Errors  []CloudflareError `json:"errors"`
	Result  T                 `json:"result"`
}

// CloudflareError represents an error in a Cloudflare API response.
type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DoRequest performs a Cloudflare API request and deserializes the response.
func DoRequest[T any](ctx context.Context, client *CloudflareClient, method, path string, body any) (*T, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = strings.NewReader(string(jsonBody))
	}

	url := fmt.Sprintf("%s%s", client.BaseURL, path)
	httpReq, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+client.APIToken)
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		var cfResp CloudflareResponse[json.RawMessage]
		if json.Unmarshal(respBody, &cfResp) == nil && len(cfResp.Errors) > 0 {
			return nil, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, cfResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var cfResp CloudflareResponse[T]
	if err := json.Unmarshal(respBody, &cfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", cfResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("API error: success=false")
	}

	return &cfResp.Result, nil
}

// DoRequestNoBody performs a Cloudflare API DELETE request that does not return a parsed body.
func DoRequestNoBody(ctx context.Context, client *CloudflareClient, path string) error {
	url := fmt.Sprintf("%s%s", client.BaseURL, path)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+client.APIToken)

	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	return nil
}
