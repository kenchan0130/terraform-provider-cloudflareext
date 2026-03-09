package store_test

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

func TestUnitSecretsStoreDataSource_Read(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[[]testStoreResponse]{
			Success: true,
			Result: []testStoreResponse{
				{
					ID:       "store-001",
					Name:     "my-store",
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
				{
					ID:       "store-002",
					Name:     "other-store",
					Created:  "2025-01-02T00:00:00Z",
					Modified: "2025-01-02T00:00:00Z",
				},
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
data "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cloudflareext_secrets_store.test", "id", "store-001"),
					resource.TestCheckResourceAttr("data.cloudflareext_secrets_store.test", "name", "my-store"),
					resource.TestCheckResourceAttr("data.cloudflareext_secrets_store.test", "created", "2025-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("data.cloudflareext_secrets_store.test", "modified", "2025-01-01T00:00:00Z"),
				),
			},
		},
	})
}

func TestUnitSecretsStoreDataSource_NotFound(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[[]testStoreResponse]{
			Success: true,
			Result:  []testStoreResponse{},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
data "cloudflareext_secrets_store" "test" {
  name = "nonexistent"
}
`),
				ExpectError: regexp.MustCompile(`Secrets Store Not Found`),
			},
		},
	})
}

func TestUnitSecretsStoreDataSource_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
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
data "cloudflareext_secrets_store" "test" {
  name = "my-store"
}
`),
				ExpectError: regexp.MustCompile(`Authentication error`),
			},
		},
	})
}
