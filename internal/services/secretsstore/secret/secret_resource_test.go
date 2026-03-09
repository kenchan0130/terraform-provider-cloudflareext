package secret_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jarcoal/httpmock"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

// testSecretCreateRequest matches the Cloudflare Secrets Store secret create request format.
// See: https://developers.cloudflare.com/secrets-store/manage-secrets/how-to/
type testSecretCreateRequest struct {
	Name    string   `json:"name"`
	Value   string   `json:"value"`
	Scopes  []string `json:"scopes"`
	Comment string   `json:"comment,omitempty"`
}

// testSecretUpdateRequest matches the Cloudflare Secrets Store secret update (PATCH) request format.
// See: https://developers.cloudflare.com/secrets-store/manage-secrets/how-to/
type testSecretUpdateRequest struct {
	Name    string   `json:"name,omitempty"`
	Value   string   `json:"value,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

// testSecretResponse matches the Cloudflare Secrets Store secret API response format.
// The value field is never returned in responses.
type testSecretResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	StoreID  string `json:"store_id"`
	Comment  string `json:"comment"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

func setupSecretMock() {
	// POST /accounts/{account_id}/secrets_store/stores/{store_id}/secrets
	// Creates a single secret.
	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body testSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			resp := shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     body.Name,
					Status:   "active",
					StoreID:  "store-001",
					Comment:  body.Comment,
					Created:  "2025-01-01T00:00:00.000000Z",
					Modified: "2025-01-01T00:00:00.000000Z",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	// GET /accounts/{account_id}/secrets_store/stores/{store_id}/secrets/{secret_id}
	// Returns secret metadata (value is never included in response).
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
			Success: true,
			Result: testSecretResponse{
				ID:       "secret-001",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-001",
				Comment:  "test secret",
				Created:  "2025-01-01T00:00:00.000000Z",
				Modified: "2025-01-01T00:00:00.000000Z",
			},
		}),
	)

	// PATCH /accounts/{account_id}/secrets_store/stores/{store_id}/secrets/{secret_id}
	// Updates secret fields. Only provided fields are updated.
	httpmock.RegisterResponder(http.MethodPatch,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(req *http.Request) (*http.Response, error) {
			var body testSecretUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}

			name := body.Name
			if name == "" {
				name = "MY_SECRET"
			}
			comment := body.Comment
			if comment == "" {
				comment = "test secret"
			}

			resp := shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     name,
					Status:   "active",
					StoreID:  "store-001",
					Comment:  comment,
					Created:  "2025-01-01T00:00:00.000000Z",
					Modified: "2025-01-02T00:00:00.000000Z",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	// DELETE /accounts/{account_id}/secrets_store/stores/{store_id}/secrets/{secret_id}
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
	)
}

func TestUnitSecretsStoreSecret_Create(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "id", "secret-001"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "name", "MY_SECRET"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-001"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "status", "active"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "test secret"),
				),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_Update(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	updatedGetRegistered := false

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "test secret"),
			},
			{
				PreConfig: func() {
					if !updatedGetRegistered {
						httpmock.RegisterResponder(http.MethodGet,
							"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
							httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
								Success: true,
								Result: testSecretResponse{
									ID:       "secret-001",
									Name:     "MY_SECRET",
									Status:   "active",
									StoreID:  "store-001",
									Comment:  "updated comment",
									Created:  "2025-01-01T00:00:00.000000Z",
									Modified: "2025-01-02T00:00:00.000000Z",
								},
							}),
						)
						updatedGetRegistered = true
					}
				},
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "new-secret-value"
  comment  = "updated comment"
  scopes   = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "updated comment"),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_ValueRequired(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  scopes   = ["workers"]
}
`),
				ExpectError: regexp.MustCompile(`one \(and only one\) of`),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_MultipleScopes(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers", "ai_gateway"]
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.#", "2"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.0", "workers"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.1", "ai_gateway"),
				),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets",
		httpmock.NewJsonResponderOrPanic(403, shared.CloudflareResponse[json.RawMessage]{
			Success: false,
			Errors: []shared.CloudflareError{
				{Code: 10000, Message: "Authentication error"},
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  scopes   = ["workers"]
}
`),
				ExpectError: regexp.MustCompile(`Authentication error`),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_StoreIDRequiresReplace(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	// Register mocks for second store
	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body testSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-002",
					Name:     body.Name,
					Status:   "active",
					StoreID:  "store-002",
					Comment:  body.Comment,
					Created:  "2025-01-01T00:00:00.000000Z",
					Modified: "2025-01-01T00:00:00.000000Z",
				},
			})
		},
	)
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets/secret-002",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
			Success: true,
			Result: testSecretResponse{
				ID:       "secret-002",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-002",
				Comment:  "test secret",
				Created:  "2025-01-01T00:00:00.000000Z",
				Modified: "2025-01-01T00:00:00.000000Z",
			},
		}),
	)
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets/secret-002",
		httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-001"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-002"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-002"),
			},
		},
	})
}
