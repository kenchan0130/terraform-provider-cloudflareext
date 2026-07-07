package destination

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

// apiError represents a non-2xx response from the Workers Observability
// destination API. It preserves the HTTP status code so callers can detect
// specific conditions (e.g. 404 Not Found) via shared.IsNotFoundError,
// without changing the error's string representation.
type apiError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s: %s", e.Status, e.Message)
}

// HTTPStatusCode implements shared.HTTPStatusCoder.
func (e *apiError) HTTPStatusCode() int {
	return e.StatusCode
}

func doRequest[T any](ctx context.Context, client *shared.CloudflareClient, method, path string, body any) (*T, error) {
	result, _, err := doRequestWithResultInfo[T](ctx, client, method, path, body)
	return result, err
}

func doRequestWithResultInfo[T any](ctx context.Context, client *shared.CloudflareClient, method, path string, body any) (*T, *shared.ResultInfo, error) {
	var requestBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, joinURL(client.BaseURL, path), requestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+client.APIToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var envelope shared.CloudflareResponse[json.RawMessage]
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return nil, nil, fmt.Errorf("failed to decode response body: %w", err)
		}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !envelope.Success {
		return nil, nil, &apiError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Message:    cloudflareErrorMessage(envelope.Errors),
		}
	}

	var result T
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return &result, envelope.ResultInfo, nil
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response result: %w", err)
	}
	return &result, envelope.ResultInfo, nil
}

func doRequestNoBody(ctx context.Context, client *shared.CloudflareClient, method, path string, body any) error {
	_, err := doRequest[json.RawMessage](ctx, client, method, path, body)
	return err
}

func joinURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func paginatedPath(path string, page int) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%spage=%d", path, separator, page)
}

func cloudflareErrorMessage(errors []shared.CloudflareError) string {
	if len(errors) == 0 {
		return "Cloudflare API request failed"
	}
	messages := make([]string, 0, len(errors))
	for _, apiErr := range errors {
		messages = append(messages, apiErr.Message)
	}
	return strings.Join(messages, "; ")
}
