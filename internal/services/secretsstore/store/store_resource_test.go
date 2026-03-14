package store_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jarcoal/httpmock"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

// testStoreCreateRequest matches the Cloudflare Secrets Store API create request format.
// The API accepts an array of create requests.
// See: https://developers.cloudflare.com/api/resources/secrets_store/subresources/stores/methods/create/
type testStoreCreateRequest struct {
	Name string `json:"name"`
}

// testStoreResponse matches the Cloudflare Secrets Store API response format.
// See: https://developers.cloudflare.com/api/resources/secrets_store/subresources/stores/methods/list/
type testStoreResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

// newPaginatedListResponder creates a responder for paginated list endpoints.
// It returns the given stores on page 1, and an empty result on subsequent pages.
func newPaginatedListResponder(stores []testStoreResponse) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		page := 1
		if p := req.URL.Query().Get("page"); p != "" {
			page, _ = strconv.Atoi(p)
		}
		if page > 1 {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[[]testStoreResponse]{
				Success: true,
				Result:  []testStoreResponse{},
				ResultInfo: &shared.ResultInfo{
					Page:       page,
					PerPage:    20,
					TotalPages: 1,
					Count:      0,
					TotalCount: len(stores),
				},
			})
		}
		return httpmock.NewJsonResponse(200, shared.CloudflareResponse[[]testStoreResponse]{
			Success: true,
			Result:  stores,
			ResultInfo: &shared.ResultInfo{
				Page:       1,
				PerPage:    20,
				TotalPages: 1,
				Count:      len(stores),
				TotalCount: len(stores),
			},
		})
	}
}

func setupStoreStoreMock() {
	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores"

	// POST /accounts/{account_id}/secrets_store/stores
	httpmock.RegisterResponder(http.MethodPost, baseURL,
		func(req *http.Request) (*http.Response, error) {
			var body []testStoreCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			if len(body) == 0 {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"empty request"}]}`), nil
			}
			resp := shared.CloudflareResponse[[]testStoreResponse]{
				Success: true,
				Result: []testStoreResponse{
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

	// GET /accounts/{account_id}/secrets_store/stores
	httpmock.RegisterResponder(http.MethodGet, baseURL,
		newPaginatedListResponder([]testStoreResponse{
			{
				ID:       "store-001",
				Name:     "my-store",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)

	// DELETE /accounts/{account_id}/secrets_store/stores/{store_id}
	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/store-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testStoreResponse]{
			Success: true,
			Result: testStoreResponse{
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

	setupStoreStoreMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-001"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "name", "my-store"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "created", "2025-01-01T00:00:00Z"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "modified", "2025-01-01T00:00:00Z"),
				),
			},
		},
	})
}

func TestUnitSecretsStore_NameRequiresReplace(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupStoreStoreMock()

	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores"

	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/store-002",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testStoreResponse]{
			Success: true,
			Result:  testStoreResponse{ID: "store-002", Name: "my-new-store"},
		}),
	)

	secondStoreCreated := false

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "name", "my-store"),
			},
			{
				PreConfig: func() {
					if !secondStoreCreated {
						httpmock.RegisterResponder(http.MethodPost, baseURL,
							func(req *http.Request) (*http.Response, error) {
								var body []testStoreCreateRequest
								if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
									return httpmock.NewStringResponse(400, ""), nil
								}
								return httpmock.NewJsonResponse(200, shared.CloudflareResponse[[]testStoreResponse]{
									Success: true,
									Result: []testStoreResponse{
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
						httpmock.RegisterResponder(http.MethodGet, baseURL,
							newPaginatedListResponder([]testStoreResponse{
								{
									ID:       "store-002",
									Name:     "my-new-store",
									Created:  "2025-01-02T00:00:00Z",
									Modified: "2025-01-02T00:00:00Z",
								},
							}),
						)
						secondStoreCreated = true
					}
				},
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-new-store"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-002"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "name", "my-new-store"),
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
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				ExpectError: regexp.MustCompile(`403 Forbidden`),
			},
		},
	})
}

func TestUnitSecretsStore_NotFoundOnRead(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupStoreStoreMock()

	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores"
	readCount := 0

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-001"),
			},
			{
				PreConfig: func() {
					if readCount == 0 {
						httpmock.RegisterResponder(http.MethodGet, baseURL,
							newPaginatedListResponder([]testStoreResponse{}),
						)
						httpmock.RegisterResponder(http.MethodDelete, baseURL+"/store-003",
							httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testStoreResponse]{
								Success: true,
								Result:  testStoreResponse{ID: "store-003", Name: "my-store"},
							}),
						)
						httpmock.RegisterResponder(http.MethodPost, baseURL,
							func(req *http.Request) (*http.Response, error) {
								var body []testStoreCreateRequest
								if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
									return httpmock.NewStringResponse(400, ""), nil
								}
								httpmock.RegisterResponder(http.MethodGet, baseURL,
									newPaginatedListResponder([]testStoreResponse{
										{
											ID:       "store-003",
											Name:     body[0].Name,
											Created:  "2025-01-03T00:00:00Z",
											Modified: "2025-01-03T00:00:00Z",
										},
									}),
								)
								return httpmock.NewJsonResponse(200, shared.CloudflareResponse[[]testStoreResponse]{
									Success: true,
									Result: []testStoreResponse{
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
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store.test", "id", "store-003"),
			},
		},
	})
}
