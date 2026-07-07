package secret_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/jarcoal/httpmock"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

// testSecretCreateRequest matches the Cloudflare Secrets Store secret create request format.
// The SDK sends an array of secret bodies.
type testSecretCreateRequest struct {
	Name   string   `json:"name"`
	Value  string   `json:"value"`
	Scopes []string `json:"scopes"`
}

// testSecretEditRequest matches the Cloudflare Secrets Store secret edit (PATCH) request format.
// Comment uses a pointer so the responder can distinguish "field omitted" from
// "field explicitly sent as an empty string".
type testSecretEditRequest struct {
	Name    string   `json:"name"`
	Value   string   `json:"value,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
	Comment *string  `json:"comment,omitempty"`
}

// testSecretResponse matches the Cloudflare Secrets Store secret API response format.
type testSecretResponse struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	StoreID  string   `json:"store_id"`
	Comment  string   `json:"comment"`
	Scopes   []string `json:"scopes"`
	Created  string   `json:"created"`
	Modified string   `json:"modified"`
}

func setupSecretMock() {
	// The GET responder echoes the scopes/comment last sent via POST/PATCH so that
	// the post-apply refresh sees the same values the configuration requested.
	currentScopes := []string{"workers"}
	currentComment := "test secret"
	// POST /accounts/{account_id}/secrets_store/stores/{store_id}/secrets
	// The SDK sends an array of secret bodies and expects an array result (SinglePage).
	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body []testSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			if len(body) == 0 {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"empty request"}]}`), nil
			}
			currentScopes = body[0].Scopes
			resp := shared.CloudflareResponse[[]testSecretResponse]{
				Success: true,
				Result: []testSecretResponse{
					{
						ID:       "secret-001",
						Name:     body[0].Name,
						Status:   "active",
						StoreID:  "store-001",
						Comment:  "test secret",
						Scopes:   body[0].Scopes,
						Created:  "2025-01-01T00:00:00Z",
						Modified: "2025-01-01T00:00:00Z",
					},
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	// GET /accounts/{account_id}/secrets_store/stores/{store_id}/secrets/{secret_id}
	// Returns secret metadata (value is never included in GET response).
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     "MY_SECRET",
					Status:   "active",
					StoreID:  "store-001",
					Comment:  currentComment,
					Scopes:   currentScopes,
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
			})
		},
	)

	// PATCH /accounts/{account_id}/secrets_store/stores/{store_id}/secrets/{secret_id}
	// Updates secret fields via the SDK's Edit method.
	httpmock.RegisterResponder(http.MethodPatch,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(req *http.Request) (*http.Response, error) {
			var body testSecretEditRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			name := body.Name
			if name == "" {
				name = "MY_SECRET"
			}
			if body.Scopes != nil {
				currentScopes = body.Scopes
			}
			// A PATCH is a partial update: only update the stored comment when
			// the request explicitly includes the field (even as an empty
			// string to clear it). An omitted field keeps the existing value.
			if body.Comment != nil {
				currentComment = *body.Comment
			}
			resp := shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     name,
					Status:   "active",
					StoreID:  "store-001",
					Comment:  currentComment,
					Scopes:   currentScopes,
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-02T00:00:00Z",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	// DELETE /accounts/{account_id}/secrets_store/stores/{store_id}/secrets/{secret_id}
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
			Success: true,
			Result: testSecretResponse{
				ID:       "secret-001",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-001",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
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
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
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

func TestUnitSecretsStoreSecret_DeleteNotFoundSucceeds(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	// Simulate the secret already being deleted out-of-band: the delete
	// endpoint now returns 404. Destroy must still succeed.
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewJsonResponderOrPanic(404, shared.CloudflareResponse[json.RawMessage]{
			Success: false,
			Errors: []shared.CloudflareError{
				{Code: 10007, Message: "secret not found"},
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "id", "secret-001"),
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
  store_id         = "store-001"
  name             = "MY_SECRET"
  value_wo         = "my-secret-value"
  value_wo_version = "1"
  comment          = "test secret"
  scopes           = ["workers"]
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
									Name:     "MY_SECRET_UPDATED",
									Status:   "active",
									StoreID:  "store-001",
									Comment:  "test secret",
									Scopes:   []string{"workers"},
									Created:  "2025-01-01T00:00:00Z",
									Modified: "2025-01-02T00:00:00Z",
								},
							}),
						)
						updatedGetRegistered = true
					}
				},
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id         = "store-001"
  name             = "MY_SECRET_UPDATED"
  value_wo         = "new-secret-value"
  value_wo_version = "2"
  comment          = "test secret"
  scopes           = ["workers"]
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "name", "MY_SECRET_UPDATED"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "value_wo_version", "2"),
				),
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
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers", "ai_gateway"]
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

// TestUnitSecretsStoreSecret_ScopeHyphenUnderscoreEquivalence verifies that when
// the Cloudflare API echoes back scopes using hyphens (e.g. "ai-gateway")
// while the configuration uses the documented underscore form ("ai_gateway"),
// and additionally returns them in a different order than the configuration,
// the provider treats them as equivalent: Create succeeds without a
// "Provider produced inconsistent result after apply" error, state keeps the
// configuration's spelling and order, and a subsequent refresh produces no diff.
func TestUnitSecretsStoreSecret_ScopeHyphenUnderscoreEquivalence(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Reversed order and hyphenated spelling compared to the configuration's
	// scopes = ["workers", "ai_gateway"].
	currentScopes := []string{"ai-gateway", "workers"}

	// POST echoes back the hyphenated form regardless of what was requested,
	// simulating an API that normalizes scopes to hyphenated spelling.
	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets",
		func(req *http.Request) (*http.Response, error) {
			var body []testSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			if len(body) == 0 {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"empty request"}]}`), nil
			}
			resp := shared.CloudflareResponse[[]testSecretResponse]{
				Success: true,
				Result: []testSecretResponse{
					{
						ID:       "secret-001",
						Name:     body[0].Name,
						Status:   "active",
						StoreID:  "store-001",
						Comment:  "test secret",
						Scopes:   currentScopes,
						Created:  "2025-01-01T00:00:00Z",
						Modified: "2025-01-01T00:00:00Z",
					},
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     "MY_SECRET",
					Status:   "active",
					StoreID:  "store-001",
					Comment:  "test secret",
					Scopes:   currentScopes,
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
			})
		},
	)

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
			Success: true,
			Result: testSecretResponse{
				ID:       "secret-001",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-001",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers", "ai_gateway"]
}
`),
				// Create must succeed (no "inconsistent result after apply") and
				// state must retain the configuration's underscore spelling and order.
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.#", "2"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.0", "workers"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.1", "ai_gateway"),
				),
			},
			{
				// A refresh-only plan must not detect any drift even though the
				// API keeps echoing the hyphenated form.
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.#", "2"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.0", "workers"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.1", "ai_gateway"),
				),
			},
		},
	})
}

func TestUnitSecretsStoreSecret_ReadNotFoundRemovesResource(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
}
`),
			},
			{
				PreConfig: func() {
					// Simulate an out-of-band deletion: the secret no longer exists.
					httpmock.RegisterResponder(http.MethodGet,
						"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
						httpmock.NewJsonResponderOrPanic(404, shared.CloudflareResponse[json.RawMessage]{
							Success: false,
							Errors: []shared.CloudflareError{
								{Code: 10007, Message: "secret not found"},
							},
						}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_secrets_store_secret.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestUnitSecretsStoreSecret_ScopesDriftDetected(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
}
`),
			},
			{
				PreConfig: func() {
					// Simulate an out-of-band change of the scopes.
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
								Scopes:   []string{"workers", "ai_gateway"},
								Created:  "2025-01-01T00:00:00Z",
								Modified: "2025-01-02T00:00:00Z",
							},
						}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.#", "2"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.0", "workers"),
					testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "scopes.1", "ai_gateway"),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_secrets_store_secret.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestUnitSecretsStoreSecret_CommentRemovedDriftDetected(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
}
`),
			},
			{
				PreConfig: func() {
					// Simulate an out-of-band removal of the comment.
					httpmock.RegisterResponder(http.MethodGet,
						"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
						httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
							Success: true,
							Result: testSecretResponse{
								ID:       "secret-001",
								Name:     "MY_SECRET",
								Status:   "active",
								StoreID:  "store-001",
								Scopes:   []string{"workers"},
								Created:  "2025-01-01T00:00:00Z",
								Modified: "2025-01-02T00:00:00Z",
							},
						}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              resource.TestCheckNoResourceAttr("cloudflareext_secrets_store_secret.test", "comment"),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_secrets_store_secret.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestUnitSecretsStoreSecret_CommentRemovedFromConfigClearsRemoteComment(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupSecretMock()

	var lastPatchBody testSecretEditRequest
	commentCleared := false

	httpmock.RegisterResponder(http.MethodPatch,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(req *http.Request) (*http.Response, error) {
			var body testSecretEditRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			lastPatchBody = body
			comment := "test secret"
			if body.Comment != nil {
				comment = *body.Comment
				commentCleared = comment == ""
			}
			resp := shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     "MY_SECRET",
					Status:   "active",
					StoreID:  "store-001",
					Comment:  comment,
					Scopes:   []string{"workers"},
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-02T00:00:00Z",
				},
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-001/secrets/secret-001",
		func(_ *http.Request) (*http.Response, error) {
			comment := "test secret"
			if commentCleared {
				comment = ""
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[testSecretResponse]{
				Success: true,
				Result: testSecretResponse{
					ID:       "secret-001",
					Name:     "MY_SECRET",
					Status:   "active",
					StoreID:  "store-001",
					Comment:  comment,
					Scopes:   []string{"workers"},
					Created:  "2025-01-01T00:00:00Z",
					Modified: "2025-01-01T00:00:00Z",
				},
			})
		},
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id         = "store-001"
  name             = "MY_SECRET"
  value_wo         = "my-secret-value"
  value_wo_version = "1"
  comment          = "test secret"
  scopes           = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "comment", "test secret"),
			},
			{
				// Removing `comment` from config must clear it remotely (via an
				// explicit empty-string PATCH), not just leave the stale value
				// on the API side while the plan shows null.
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id         = "store-001"
  name             = "MY_SECRET"
  value_wo         = "my-secret-value"
  value_wo_version = "1"
  scopes           = ["workers"]
}
`),
				Check: resource.TestCheckNoResourceAttr("cloudflareext_secrets_store_secret.test", "comment"),
			},
		},
	})

	if lastPatchBody.Comment == nil {
		t.Fatalf("expected PATCH request to explicitly include the comment field, got none")
	}
	if *lastPatchBody.Comment != "" {
		t.Fatalf("expected PATCH request to send an empty comment, got %q", *lastPatchBody.Comment)
	}
}

func TestUnitSecretsStoreSecret_ValueWORequiresVersion(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id = "store-001"
  name     = "MY_SECRET"
  value_wo = "my-secret-value"
  scopes   = ["workers"]
}
`),
				ExpectError: regexp.MustCompile(`value_wo_version`),
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
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  scopes          = ["workers"]
}
`),
				ExpectError: regexp.MustCompile(`403 Forbidden`),
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
			var body []testSecretCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			if len(body) == 0 {
				return httpmock.NewStringResponse(400, ""), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[[]testSecretResponse]{
				Success: true,
				Result: []testSecretResponse{
					{
						ID:       "secret-002",
						Name:     body[0].Name,
						Status:   "active",
						StoreID:  "store-002",
						Comment:  "test secret",
						Scopes:   body[0].Scopes,
						Created:  "2025-01-01T00:00:00Z",
						Modified: "2025-01-01T00:00:00Z",
					},
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
				Scopes:   []string{"workers"},
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/secrets_store/stores/store-002/secrets/secret-002",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testSecretResponse]{
			Success: true,
			Result: testSecretResponse{
				ID:       "secret-002",
				Name:     "MY_SECRET",
				Status:   "active",
				StoreID:  "store-002",
				Created:  "2025-01-01T00:00:00Z",
				Modified: "2025-01-01T00:00:00Z",
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-001"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-001"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_secrets_store_secret" "test" {
  store_id        = "store-002"
  name            = "MY_SECRET"
  value_wo        = "my-secret-value"
  value_wo_version = "1"
  comment         = "test secret"
  scopes          = ["workers"]
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_secrets_store_secret.test", "store_id", "store-002"),
			},
		},
	})
}
