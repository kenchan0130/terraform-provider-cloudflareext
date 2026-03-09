package provider

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jarcoal/httpmock"
)

func setupSecretsStoreMock() {
	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body apiSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			resp := cloudflareResponse[apiSecretResponse]{
				Success: true,
				Result: apiSecretResponse{
					ID:       "secret-001",
					Name:     body.Name,
					Status:   "active",
					StoreID:  "store-001",
					Comment:  body.Comment,
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[apiSecretResponse]{
			Success: true,
			Result: apiSecretResponse{
				ID:       "secret-001",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-001",
				Comment:  "test secret",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)

	httpmock.RegisterResponder(http.MethodPatch,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(req *http.Request) (*http.Response, error) {
			var body apiSecretUpdateRequest
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

			resp := cloudflareResponse[apiSecretResponse]{
				Success: true,
				Result: apiSecretResponse{
					ID:       "secret-001",
					Name:     name,
					Status:   "active",
					StoreID:  "store-001",
					Comment:  comment,
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-02T00:00:00Z",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
	)
}

func TestUnitSecretsStoreSecret_Create(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretsStoreMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "id", "secret-001"),
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "name", "MY_SECRET"),
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-001"),
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "status", "active"),
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "test secret"),
				),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_Update(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretsStoreMock()

	updatedGetRegistered := false

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "test secret"),
			},
			{
				PreConfig: func() {
					if !updatedGetRegistered {
						httpmock.RegisterResponder(http.MethodGet,
							"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
							httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[apiSecretResponse]{
								Success: true,
								Result: apiSecretResponse{
									ID:       "secret-001",
									Name:     "MY_SECRET",
									Status:   "active",
									StoreID:  "store-001",
									Comment:  "updated comment",
									Created:  "2025-01-01T00:00:00Z",
									Modified: "2025-01-02T00:00:00Z",
								},
							}),
						)
						updatedGetRegistered = true
					}
				},
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "new-secret-value"
  comment  = "updated comment"
  scopes   = ["workers"]
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "updated comment"),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_ValueRequired(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
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

	setupSecretsStoreMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers", "ai_gateway"]
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.#", "2"),
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.0", "workers"),
					testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.1", "ai_gateway"),
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
		httpmock.NewJsonResponderOrPanic(403, cloudflareResponse[json.RawMessage]{
			Success: false,
			Errors: []cloudflareError{
				{Code: 10000, Message: "Authentication error"},
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
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

	setupSecretsStoreMock()

	// Register mocks for second store
	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body apiSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			return httpmock.NewJsonResponse(200, cloudflareResponse[apiSecretResponse]{
				Success: true,
				Result: apiSecretResponse{
					ID:       "secret-002",
					Name:     body.Name,
					Status:   "active",
					StoreID:  "store-002",
					Comment:  body.Comment,
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
			})
		},
	)
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets/secret-002",
		httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[apiSecretResponse]{
			Success: true,
			Result: apiSecretResponse{
				ID:       "secret-002",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-002",
				Comment:  "test secret",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets/secret-002",
		httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-001"),
			},
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-002"
  name     = "MY_SECRET"
  value_wo = "super-secret-value"
  comment  = "test secret"
  scopes   = ["workers"]
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-002"),
			},
		},
	})
}
