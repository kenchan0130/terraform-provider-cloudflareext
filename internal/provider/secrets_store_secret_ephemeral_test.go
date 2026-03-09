package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/jarcoal/httpmock"
)

func TestUnitSecretsStoreSecretEphemeral_Metadata(t *testing.T) {
	e := &SecretsStoreSecretEphemeral{}

	var resp ephemeral.MetadataResponse
	e.Metadata(context.Background(), ephemeral.MetadataRequest{ProviderTypeName: "cloudflareext"}, &resp)

	if resp.TypeName != "cloudflareext_secrets_store_secret" {
		t.Errorf("expected type name 'cloudflareext_secrets_store_secret', got '%s'", resp.TypeName)
	}
}

func TestUnitSecretsStoreSecretEphemeral_Schema(t *testing.T) {
	e := &SecretsStoreSecretEphemeral{}

	var resp ephemeral.SchemaResponse
	e.Schema(context.Background(), ephemeral.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema errors: %v", resp.Diagnostics)
	}

	attrs := resp.Schema.Attributes
	requiredAttrs := []string{"store_id", "secret_id"}
	for _, name := range requiredAttrs {
		attr, ok := attrs[name]
		if !ok {
			t.Errorf("expected attribute '%s' to exist", name)
			continue
		}
		if !attr.IsRequired() {
			t.Errorf("expected attribute '%s' to be required", name)
		}
	}

	computedAttrs := []string{"name", "status", "comment", "created", "modified"}
	for _, name := range computedAttrs {
		attr, ok := attrs[name]
		if !ok {
			t.Errorf("expected attribute '%s' to exist", name)
			continue
		}
		if !attr.IsComputed() {
			t.Errorf("expected attribute '%s' to be computed", name)
		}
	}
}

func TestUnitSecretsStoreSecretEphemeral_APICall(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.example.com/accounts/test-account/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[apiSecretResponse]{
			Success: true,
			Result: apiSecretResponse{
				ID:       "secret-001",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-001",
				Comment:  "test comment",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	apiPath := "/accounts/test-account/secrets_store/stores/store-001/secrets/secret-001"
	result, err := doRequest[apiSecretResponse](context.Background(), client, http.MethodGet, apiPath, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Name != "MY_SECRET" {
		t.Errorf("expected name 'MY_SECRET', got '%s'", result.Name)
	}
	if result.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", result.Status)
	}
	if result.Comment != "test comment" {
		t.Errorf("expected comment 'test comment', got '%s'", result.Comment)
	}
}

func TestUnitSecretsStoreSecretEphemeral_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.example.com/accounts/test-account/secrets_store/stores/store-001/secrets/not-found",
		httpmock.NewJsonResponderOrPanic(404, cloudflareResponse[any]{
			Success: false,
			Errors: []cloudflareError{
				{Code: 7003, Message: "Secret not found"},
			},
		}),
	)

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.example.com",
		APIToken:   "test-token",
		AccountID:  "test-account",
	}

	apiPath := "/accounts/test-account/secrets_store/stores/store-001/secrets/not-found"
	_, err := doRequest[apiSecretResponse](context.Background(), client, http.MethodGet, apiPath, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
