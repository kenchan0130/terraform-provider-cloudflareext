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

// testScriptSecretUpdateRequest matches the Cloudflare Workers Script secret PUT request format.
type testScriptSecretUpdateRequest struct {
	Name string `json:"name"`
	Text string `json:"text"`
	Type string `json:"type"`
}

// testScriptSecretResponse matches the Cloudflare Workers Script secret API response format.
type testScriptSecretResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func setupScriptSecretMock() {
	// PUT /accounts/{account_id}/workers/scripts/{script_name}/secrets
	// The SDK uses PUT for both create and update (upsert).
	httpmock.RegisterResponder(http.MethodPut,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/my-worker/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body testScriptSecretUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			resp := shared.CloudflareResponse[testScriptSecretResponse]{
				Success: true,
				Result: testScriptSecretResponse{
					Name: body.Name,
					Type: "secret_text",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	// GET /accounts/{account_id}/workers/scripts/{script_name}/secrets/{secret_name}
	// Returns secret metadata (value is never included in GET response).
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/my-worker/secrets/MY_SECRET",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testScriptSecretResponse]{
			Success: true,
			Result: testScriptSecretResponse{
				Name: "MY_SECRET",
				Type: "secret_text",
			},
		}),
	)

	// DELETE /accounts/{account_id}/workers/scripts/{script_name}/secrets/{secret_name}
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/my-worker/secrets/MY_SECRET",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[json.RawMessage]{
			Success: true,
		}),
	)
}

func TestUnitWorkersScriptSecret_Create(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupScriptSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "name", "MY_SECRET"),
					testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "script_name", "my-worker"),
					testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "type", "secret_text"),
				),
			},
		},
	})
}

func TestUnitWorkersScriptSecret_Update(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupScriptSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "text_wo_version", "1"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_SECRET"
  text_wo          = "new-secret-value"
  text_wo_version  = "2"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "text_wo_version", "2"),
			},
		},
	})
}

func TestUnitWorkersScriptSecret_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPut,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/my-worker/secrets",
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
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				ExpectError: regexp.MustCompile(`403 Forbidden`),
			},
		},
	})
}

func TestUnitWorkersScriptSecret_TextWORequiresVersion(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name = "my-worker"
  name        = "MY_SECRET"
  text_wo     = "my-secret-value"
}
`),
				ExpectError: regexp.MustCompile(`text_wo_version`),
			},
		},
	})
}

func TestUnitWorkersScriptSecret_ScriptNameRequiresReplace(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupScriptSecretMock()

	// Register mocks for second script
	httpmock.RegisterResponder(http.MethodPut,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/other-worker/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body testScriptSecretUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[testScriptSecretResponse]{
				Success: true,
				Result: testScriptSecretResponse{
					Name: body.Name,
					Type: "secret_text",
				},
			})
		},
	)
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/other-worker/secrets/MY_SECRET",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testScriptSecretResponse]{
			Success: true,
			Result: testScriptSecretResponse{
				Name: "MY_SECRET",
				Type: "secret_text",
			},
		}),
	)
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/other-worker/secrets/MY_SECRET",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[json.RawMessage]{
			Success: true,
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "script_name", "my-worker"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "other-worker"
  name             = "MY_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "script_name", "other-worker"),
			},
		},
	})
}

func TestUnitWorkersScriptSecret_NameRequiresReplace(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupScriptSecretMock()

	// Register mocks for the renamed secret
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/my-worker/secrets/MY_OTHER_SECRET",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testScriptSecretResponse]{
			Success: true,
			Result: testScriptSecretResponse{
				Name: "MY_OTHER_SECRET",
				Type: "secret_text",
			},
		}),
	)
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/scripts/my-worker/secrets/MY_OTHER_SECRET",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[json.RawMessage]{
			Success: true,
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "name", "MY_SECRET"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_script_secret" "test" {
  script_name      = "my-worker"
  name             = "MY_OTHER_SECRET"
  text_wo          = "my-secret-value"
  text_wo_version  = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_script_secret.test", "name", "MY_OTHER_SECRET"),
			},
		},
	})
}
