package provider

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jarcoal/httpmock"
)

func setupSecretsStoreStoreMock() {
	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores"

	httpmock.RegisterResponder(http.MethodPost, baseURL,
		func(req *http.Request) (*http.Response, error) {
			var body []apiStoreCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			if len(body) == 0 {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"empty request"}]}`), nil
			}
			resp := cloudflareResponse[[]apiStoreResponse]{
				Success: true,
				Result: []apiStoreResponse{
					{
						ID:       "store-001",
						Name:     body[0].Name,
						Created:  "2025-01-01T00:00:00Z",
						Modified: "2025-01-01T00:00:00Z",
					},
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodGet, baseURL,
		httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[[]apiStoreResponse]{
			Success: true,
			Result: []apiStoreResponse{
				{
					ID:       "store-001",
					Name:     "my-store",
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
			},
		}),
	)

	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/store-001",
		httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[apiStoreResponse]{
			Success: true,
			Result: apiStoreResponse{
				ID:       "store-001",
				Name:     "my-store",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)
}

func TestUnitSecretsStore_Create(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretsStoreStoreMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-001"),
					testCheckResourceAttr("cloudflareext_secrets_store.test", "name", "my-store"),
					testCheckResourceAttr("cloudflareext_secrets_store.test", "created", "2025-01-01T00:00:00Z"),
					testCheckResourceAttr("cloudflareext_secrets_store.test", "modified", "2025-01-01T00:00:00Z"),
				),
			},
		},
	})
}

func TestUnitSecretsStore_NameRequiresReplace(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretsStoreStoreMock()

	// Register mocks for second store
	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores"

	secondStoreCreated := false

	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/store-002",
		httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[apiStoreResponse]{
			Success: true,
			Result: apiStoreResponse{
				ID:   "store-002",
				Name: "my-new-store",
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store.test", "name", "my-store"),
			},
			{
				PreConfig: func() {
					if !secondStoreCreated {
						// Override POST to return new store
						httpmock.RegisterResponder(http.MethodPost, baseURL,
							func(req *http.Request) (*http.Response, error) {
								var body []apiStoreCreateRequest
								if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
									return httpmock.NewStringResponse(400, ""), nil
								}
								return httpmock.NewJsonResponse(200, cloudflareResponse[[]apiStoreResponse]{
									Success: true,
									Result: []apiStoreResponse{
										{
											ID:       "store-002",
											Name:     body[0].Name,
											Created:  "2025-01-02T00:00:00Z",
											Modified: "2025-01-02T00:00:00Z",
										},
									},
								})
							},
						)
						// Override GET to return new store
						httpmock.RegisterResponder(http.MethodGet, baseURL,
							httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[[]apiStoreResponse]{
								Success: true,
								Result: []apiStoreResponse{
									{
										ID:       "store-002",
										Name:     "my-new-store",
										Created:  "2025-01-02T00:00:00Z",
										Modified: "2025-01-02T00:00:00Z",
									},
								},
							}),
						)
						secondStoreCreated = true
					}
				},
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-new-store"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-002"),
					testCheckResourceAttr("cloudflareext_secrets_store.test", "name", "my-new-store"),
				),
			},
		},
	})
}

func TestUnitSecretsStore_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores",
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
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				ExpectError: regexp.MustCompile(`Authentication error`),
			},
		},
	})
}

func TestUnitSecretsStore_NotFoundOnRead(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretsStoreStoreMock()

	readCount := 0

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testUnitTestProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-001"),
			},
			{
				PreConfig: func() {
					if readCount == 0 {
						// Return empty list to simulate store being deleted externally
						httpmock.RegisterResponder(http.MethodGet,
							"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores",
							httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[[]apiStoreResponse]{
								Success: true,
								Result:  []apiStoreResponse{},
							}),
						)
						// Register DELETE for recreated store
						httpmock.RegisterResponder(http.MethodDelete,
							"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-003",
							httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
						)
						// Re-register POST for recreation
						httpmock.RegisterResponder(http.MethodPost,
							"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores",
							func(req *http.Request) (*http.Response, error) {
								var body []apiStoreCreateRequest
								if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
									return httpmock.NewStringResponse(400, ""), nil
								}
								// After recreation, update GET to return new store
								httpmock.RegisterResponder(http.MethodGet,
									"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores",
									httpmock.NewJsonResponderOrPanic(200, cloudflareResponse[[]apiStoreResponse]{
										Success: true,
										Result: []apiStoreResponse{
											{
												ID:       "store-003",
												Name:     body[0].Name,
												Created:  "2025-01-03T00:00:00Z",
												Modified: "2025-01-03T00:00:00Z",
											},
										},
									}),
								)
								return httpmock.NewJsonResponse(200, cloudflareResponse[[]apiStoreResponse]{
									Success: true,
									Result: []apiStoreResponse{
										{
											ID:       "store-003",
											Name:     body[0].Name,
											Created:  "2025-01-03T00:00:00Z",
											Modified: "2025-01-03T00:00:00Z",
										},
									},
								})
							},
						)
						readCount++
					}
				},
				Config: testUnitTestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: testCheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-003"),
			},
		},
	})
}
