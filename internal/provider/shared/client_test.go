package shared

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
)

type testResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestDoRequest_Success(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.example.com/test",
		httpmock.NewJsonResponderOrPanic(200, CloudflareResponse[testResponse]{
			Success: true,
			Result:  testResponse{ID: "test-id", Name: "test-name"},
		}),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	result, err := DoRequest[testResponse](context.Background(), client, http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", result.ID)
	}
	if result.Name != "test-name" {
		t.Errorf("expected Name 'test-name', got '%s'", result.Name)
	}
}

func TestDoRequest_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.example.com/test",
		httpmock.NewJsonResponderOrPanic(403, map[string]any{
			"success": false,
			"errors":  []map[string]any{{"code": 10000, "message": "Forbidden"}},
		}),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	_, err := DoRequest[testResponse](context.Background(), client, http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDoRequest_WithBody(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		"https://api.example.com/test",
		httpmock.NewJsonResponderOrPanic(200, CloudflareResponse[testResponse]{
			Success: true,
			Result:  testResponse{ID: "created-id", Name: "created-name"},
		}),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	body := map[string]string{"name": "test"}
	result, err := DoRequest[testResponse](context.Background(), client, http.MethodPost, "/test", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != "created-id" {
		t.Errorf("expected ID 'created-id', got '%s'", result.ID)
	}
}

func TestDoRequestNoBody_Success(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.example.com/test",
		httpmock.NewStringResponder(204, ""),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	err := DoRequestNoBody(context.Background(), client, "/test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDoRequestNoBody_Error(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.example.com/test",
		httpmock.NewStringResponder(500, `{"success":false,"errors":[{"code":500,"message":"Internal Server Error"}]}`),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	err := DoRequestNoBody(context.Background(), client, "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDoRequest_AuthorizationHeader(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.example.com/test",
		func(req *http.Request) (*http.Response, error) {
			auth := req.Header.Get("Authorization")
			if auth != "Bearer my-secret-token" {
				t.Errorf("expected 'Bearer my-secret-token', got '%s'", auth)
			}
			return httpmock.NewJsonResponse(200, CloudflareResponse[testResponse]{
				Success: true,
				Result:  testResponse{ID: "test"},
			})
		},
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "my-secret-token",
		AccountID:  "test-account",
	}

	_, err := DoRequest[testResponse](context.Background(), client, http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
