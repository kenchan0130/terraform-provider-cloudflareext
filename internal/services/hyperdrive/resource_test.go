package hyperdrive_test

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/jarcoal/httpmock"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

type apiHyperdriveCreateRequest struct {
	Name   string `json:"name"`
	Origin struct {
		Host     string `json:"host"`
		Port     int64  `json:"port"`
		Database string `json:"database"`
		User     string `json:"user"`
		Password string `json:"password"`
		Scheme   string `json:"scheme"`
	} `json:"origin"`
}

// apiHyperdriveOriginResponse represents the origin object in a Hyperdrive API response.
type apiHyperdriveOriginResponse struct {
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Scheme   string `json:"scheme"`
}

const hyperdriveConfigsEndpoint = "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs"

var defaultHyperdriveOrigin = apiHyperdriveOriginResponse{
	Host:     "db.example.com",
	Port:     5432,
	Database: "mydb",
	User:     "dbuser",
	Scheme:   "postgresql",
}

func hyperdriveConfigEndpoint(id string) string {
	return hyperdriveConfigsEndpoint + "/" + id
}

func registerStandardDeleteResponder(id string) {
	httpmock.RegisterResponder(http.MethodDelete,
		hyperdriveConfigEndpoint(id),
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)
}

// apiHyperdriveCachingResponse represents the caching object in a Hyperdrive API response.
type apiHyperdriveCachingResponse struct {
	Disabled             bool  `json:"disabled"`
	MaxAge               int64 `json:"max_age"`
	StaleWhileRevalidate int64 `json:"stale_while_revalidate"`
}

// apiHyperdriveMTLSResponse represents the mtls object in a Hyperdrive API response.
type apiHyperdriveMTLSResponse struct {
	CACertificateID   string `json:"ca_certificate_id"`
	MTLSCertificateID string `json:"mtls_certificate_id"`
	Sslmode           string `json:"sslmode"`
}

// apiHyperdriveResponse matches the Cloudflare Hyperdrive API response format.
// See: https://developers.cloudflare.com/api/resources/hyperdrive/subresources/configs/
type apiHyperdriveResponse struct {
	ID                    string                       `json:"id"`
	Name                  string                       `json:"name"`
	Origin                apiHyperdriveOriginResponse  `json:"origin"`
	Caching               apiHyperdriveCachingResponse `json:"caching"`
	MTLS                  apiHyperdriveMTLSResponse    `json:"mtls"`
	CreatedOn             string                       `json:"created_on"`
	ModifiedOn            string                       `json:"modified_on"`
	OriginConnectionLimit int                          `json:"origin_connection_limit"`
}

func newHyperdriveResponse(id, name string, origin apiHyperdriveOriginResponse) apiHyperdriveResponse {
	return apiHyperdriveResponse{
		ID:     id,
		Name:   name,
		Origin: origin,
		Caching: apiHyperdriveCachingResponse{
			Disabled:             false,
			MaxAge:               60,
			StaleWhileRevalidate: 15,
		},
		CreatedOn:             "2025-01-01T00:00:00Z",
		ModifiedOn:            "2025-01-01T00:00:00Z",
		OriginConnectionLimit: 60,
	}
}

func setupHyperdriveMock() {
	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		func(req *http.Request) (*http.Response, error) {
			var body apiHyperdriveCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			if body.Origin.Password == "" {
				return httpmock.NewJsonResponse(400, shared.CloudflareResponse[any]{
					Success: false,
					Errors: []shared.CloudflareError{
						{Code: 2007, Message: "Invalid Hyperdrive config: origin: (password: cannot be blank.)."},
					},
				})
			}
			resp := shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result: newHyperdriveResponse("hd-test-id-001", body.Name, apiHyperdriveOriginResponse{
					Host:     body.Origin.Host,
					Port:     body.Origin.Port,
					Database: body.Origin.Database,
					User:     body.Origin.User,
					Scheme:   body.Origin.Scheme,
				}),
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  newHyperdriveResponse("hd-test-id-001", "my-hyperdrive", defaultHyperdriveOrigin),
		}),
	)

	httpmock.RegisterResponder(http.MethodPut,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		func(req *http.Request) (*http.Response, error) {
			var body apiHyperdriveCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
			}
			resp := shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result: newHyperdriveResponse("hd-test-id-001", body.Name, apiHyperdriveOriginResponse{
					Host:     body.Origin.Host,
					Port:     body.Origin.Port,
					Database: body.Origin.Database,
					User:     body.Origin.User,
					Scheme:   body.Origin.Scheme,
				}),
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	registerStandardDeleteResponder("hd-test-id-001")
}

func TestUnitHyperdriveConfig_Create(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "id", "hd-test-id-001"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "name", "my-hyperdrive"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.host", "db.example.com"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.port", "5432"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.database", "mydb"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.user", "dbuser"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.scheme", "postgresql"),
				),
			},
		},
	})
}

func TestUnitHyperdriveConfig_Update(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	updatedGetRegistered := false

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "name", "my-hyperdrive"),
			},
			{
				PreConfig: func() {
					if !updatedGetRegistered {
						httpmock.RegisterResponder(http.MethodGet,
							hyperdriveConfigEndpoint("hd-test-id-001"),
							httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
								Success: true,
								Result:  newHyperdriveResponse("hd-test-id-001", "my-hyperdrive-updated", defaultHyperdriveOrigin),
							}),
						)
						updatedGetRegistered = true
					}
				},
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive-updated"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "name", "my-hyperdrive-updated"),
			},
		},
	})
}

func TestUnitHyperdriveConfig_ReadNotFoundRemovesResource(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	config := testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
			},
			{
				PreConfig: func() {
					// Simulate an out-of-band deletion: the config no longer exists.
					httpmock.RegisterResponder(http.MethodGet,
						hyperdriveConfigEndpoint("hd-test-id-001"),
						httpmock.NewJsonResponderOrPanic(404, shared.CloudflareResponse[json.RawMessage]{
							Success: false,
							Errors: []shared.CloudflareError{
								{Code: 10007, Message: "hyperdrive config not found"},
							},
						}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_hyperdrive_config.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestUnitHyperdriveConfig_DeleteNotFoundSucceeds(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	// Simulate the config already being deleted out-of-band: the delete
	// endpoint now returns 404. Destroy must still succeed.
	httpmock.RegisterResponder(http.MethodDelete,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		httpmock.NewJsonResponderOrPanic(404, shared.CloudflareResponse[json.RawMessage]{
			Success: false,
			Errors: []shared.CloudflareError{
				{Code: 10007, Message: "hyperdrive config not found"},
			},
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "id", "hd-test-id-001"),
			},
		},
	})
}

func TestUnitHyperdriveConfig_RequiredFields(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
}
`),
				ExpectError: regexp.MustCompile(`The argument "origin" is required`),
			},
		},
	})
}

func TestUnitHyperdriveConfig_PasswordRequired(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
  }
}
`),
				ExpectError: regexp.MustCompile(`one \(and only one\) of`),
			},
		},
	})
}

func TestUnitHyperdriveConfig_PasswordWORequiresVersion(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host        = "db.example.com"
    database    = "mydb"
    user        = "dbuser"
    password_wo = "secret"
  }
}
`),
				ExpectError: regexp.MustCompile(`password_wo_version`),
			},
		},
	})
}

func TestUnitHyperdriveConfig_AccessClientSecretWORequiresVersion(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host                   = "db.example.com"
    database               = "mydb"
    user                   = "dbuser"
    password_wo            = "secret"
    password_wo_version    = "1"
    access_client_id       = "client-id"
    access_client_secret_wo = "client-secret"
  }
}
`),
				ExpectError: regexp.MustCompile(`access_client_secret_wo_version`),
			},
		},
	})
}

func TestUnitHyperdriveConfig_ImportState(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
			},
			{
				ResourceName:                         "cloudflareext_hyperdrive_config.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				// The pre-import state has caching == null: the config omits the
				// `caching` block, and Create's mapResponseToModel guard
				// (`data.Caching != nil`) leaves it unset so applied state matches
				// the plan-preserved null (see preserveCachingStateWhenOmitted,
				// issue #64). The post-import state has caching populated, because
				// Read always reflects the API's caching object into state
				// regardless of what the freshly-imported state started with
				// (issue #60). These two are expected to differ, so caching can't
				// be verified for equivalence here.
				ImportStateVerifyIgnore: []string{"origin.password", "origin.password_wo", "origin.password_wo_version", "caching"},
			},
		},
	})
}

// TestUnitHyperdriveConfig_ImportStateCachingDisabled reproduces issue #60:
// importing a Hyperdrive config whose remote caching is disabled must
// populate the `caching` block into state. Freshly imported state has only
// `id` set, so mapResponseToModel's `data.Caching != nil` guard previously
// skipped the API's caching object entirely.
func TestUnitHyperdriveConfig_ImportStateCachingDisabled(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveCachingDisabledMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = true
  }
}
`),
			},
			{
				ResourceName:                         "cloudflareext_hyperdrive_config.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				ImportStateVerifyIgnore:              []string{"origin.password", "origin.password_wo", "origin.password_wo_version"},
			},
		},
	})
}

// TestUnitHyperdriveConfig_ImportStateCachingEnabled reproduces issue #60 for
// the caching-enabled case: importing a Hyperdrive config with caching
// enabled (and server-side defaults for max_age / stale_while_revalidate)
// must populate the full `caching` block into state.
func TestUnitHyperdriveConfig_ImportStateCachingEnabled(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {}
}
`),
			},
			{
				ResourceName:                         "cloudflareext_hyperdrive_config.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				ImportStateVerifyIgnore:              []string{"origin.password", "origin.password_wo", "origin.password_wo_version"},
			},
		},
	})
}

func TestUnitHyperdriveConfig_CustomPort(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		func(req *http.Request) (*http.Response, error) {
			var body apiHyperdriveCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result: newHyperdriveResponse("hd-test-id-002", body.Name, apiHyperdriveOriginResponse{
					Host:     body.Origin.Host,
					Port:     body.Origin.Port,
					Database: body.Origin.Database,
					User:     body.Origin.User,
					Scheme:   body.Origin.Scheme,
				}),
			})
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-test-id-002"),
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result: newHyperdriveResponse("hd-test-id-002", "mysql-hyperdrive", apiHyperdriveOriginResponse{
				Host:     "mysql.example.com",
				Port:     3306,
				Database: "mydb",
				User:     "dbuser",
				Scheme:   "mysql",
			}),
		}),
	)

	registerStandardDeleteResponder("hd-test-id-002")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "mysql-hyperdrive"
  origin = {
    host     = "mysql.example.com"
    port     = 3306
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
    scheme   = "mysql"
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.port", "3306"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.scheme", "mysql"),
				),
			},
		},
	})
}

func TestUnitHyperdriveConfig_Caching(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled               = false
    max_age                = 60
    stale_while_revalidate = 15
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "false"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "60"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate", "15"),
				),
			},
			{
				PreConfig: func() {
					// Simulate the explicitly managed caching values being changed
					// out-of-band. The refresh plan must restore the configured values.
					drifted := newHyperdriveResponse("hd-test-id-001", "my-hyperdrive", defaultHyperdriveOrigin)
					drifted.Caching.MaxAge = 120
					drifted.Caching.StaleWhileRevalidate = 30
					httpmock.RegisterResponder(http.MethodGet,
						hyperdriveConfigEndpoint("hd-test-id-001"),
						httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
							Success: true,
							Result:  drifted,
						}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_hyperdrive_config.test", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(
							"cloudflareext_hyperdrive_config.test",
							tfjsonpath.New("caching").AtMapKey("max_age"),
							knownvalue.Int64Exact(60),
						),
						plancheck.ExpectKnownValue(
							"cloudflareext_hyperdrive_config.test",
							tfjsonpath.New("caching").AtMapKey("stale_while_revalidate"),
							knownvalue.Int64Exact(15),
						),
					},
				},
			},
		},
	})
}

// apiHyperdriveAccessOriginResponse represents the origin object in a Hyperdrive API
// response for an access-protected origin (behind a Cloudflare Tunnel). Per the
// Cloudflare OpenAPI schema, access-protected origins have no `port` field in
// either the request or the response.
type apiHyperdriveAccessOriginResponse struct {
	Host           string `json:"host"`
	Database       string `json:"database"`
	User           string `json:"user"`
	Scheme         string `json:"scheme"`
	AccessClientID string `json:"access_client_id"`
}

// apiHyperdriveAccessResponse matches the Cloudflare Hyperdrive API response format
// for an access-protected origin.
type apiHyperdriveAccessResponse struct {
	ID                    string                            `json:"id"`
	Name                  string                            `json:"name"`
	Origin                apiHyperdriveAccessOriginResponse `json:"origin"`
	Caching               apiHyperdriveCachingResponse      `json:"caching"`
	CreatedOn             string                            `json:"created_on"`
	ModifiedOn            string                            `json:"modified_on"`
	OriginConnectionLimit int                               `json:"origin_connection_limit"`
}

func TestUnitHyperdriveConfig_AccessProtectedOriginNoPort(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	newAccessResponse := func(id, name string, origin apiHyperdriveAccessOriginResponse) apiHyperdriveAccessResponse {
		return apiHyperdriveAccessResponse{
			ID:     id,
			Name:   name,
			Origin: origin,
			Caching: apiHyperdriveCachingResponse{
				Disabled:             false,
				MaxAge:               60,
				StaleWhileRevalidate: 15,
			},
			CreatedOn:             "2025-01-01T00:00:00Z",
			ModifiedOn:            "2025-01-01T00:00:00Z",
			OriginConnectionLimit: 60,
		}
	}

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		func(req *http.Request) (*http.Response, error) {
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			origin, _ := body["origin"].(map[string]any)
			resp := shared.CloudflareResponse[apiHyperdriveAccessResponse]{
				Success: true,
				Result: newAccessResponse("hd-access-001", body["name"].(string), apiHyperdriveAccessOriginResponse{
					Host:           origin["host"].(string),
					Database:       origin["database"].(string),
					User:           origin["user"].(string),
					Scheme:         origin["scheme"].(string),
					AccessClientID: origin["access_client_id"].(string),
				}),
			}
			return httpmock.NewJsonResponse(200, resp)
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-access-001"),
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveAccessResponse]{
			Success: true,
			Result: newAccessResponse("hd-access-001", "tunnel-hyperdrive", apiHyperdriveAccessOriginResponse{
				Host:           "db.internal.example.com",
				Database:       "mydb",
				User:           "dbuser",
				Scheme:         "postgresql",
				AccessClientID: "client-id",
			}),
		}),
	)

	registerStandardDeleteResponder("hd-access-001")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "tunnel-hyperdrive"
  origin = {
    host                    = "db.internal.example.com"
    database                = "mydb"
    user                    = "dbuser"
    password_wo             = "dbpass"
    password_wo_version     = "1"
    access_client_id        = "client-id"
    access_client_secret_wo = "client-secret"
    access_client_secret_wo_version = "1"
  }
}
`),
				// Applying must succeed (no "inconsistent result after apply" error)
				// even though the API response omits `origin.port` entirely, and the
				// schema default of 5432 should be reflected in state.
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "id", "hd-access-001"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.port", "5432"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.access_client_id", "client-id"),
				),
			},
		},
	})
}

func TestUnitHyperdriveConfig_AccessClientIDDriftDetected(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	newAccessResponse := func(id, name string, origin apiHyperdriveAccessOriginResponse) apiHyperdriveAccessResponse {
		return apiHyperdriveAccessResponse{
			ID:     id,
			Name:   name,
			Origin: origin,
			Caching: apiHyperdriveCachingResponse{
				Disabled:             false,
				MaxAge:               60,
				StaleWhileRevalidate: 15,
			},
			CreatedOn:             "2025-01-01T00:00:00Z",
			ModifiedOn:            "2025-01-01T00:00:00Z",
			OriginConnectionLimit: 60,
		}
	}

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveAccessResponse]{
			Success: true,
			Result: newAccessResponse("hd-drift-001", "tunnel-hyperdrive", apiHyperdriveAccessOriginResponse{
				Host:           "db.internal.example.com",
				Database:       "mydb",
				User:           "dbuser",
				Scheme:         "postgresql",
				AccessClientID: "client-id",
			}),
		}),
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-drift-001"),
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveAccessResponse]{
			Success: true,
			Result: newAccessResponse("hd-drift-001", "tunnel-hyperdrive", apiHyperdriveAccessOriginResponse{
				Host:           "db.internal.example.com",
				Database:       "mydb",
				User:           "dbuser",
				Scheme:         "postgresql",
				AccessClientID: "client-id",
			}),
		}),
	)

	registerStandardDeleteResponder("hd-drift-001")

	config := testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "tunnel-hyperdrive"
  origin = {
    host                    = "db.internal.example.com"
    database                = "mydb"
    user                    = "dbuser"
    password_wo             = "dbpass"
    password_wo_version     = "1"
    access_client_id        = "client-id"
    access_client_secret_wo = "client-secret"
    access_client_secret_wo_version = "1"
  }
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "origin.access_client_id", "client-id"),
			},
			{
				PreConfig: func() {
					// Simulate the origin being switched from access-protected
					// to public out-of-band: the response no longer includes
					// access_client_id.
					httpmock.RegisterResponder(http.MethodGet,
						hyperdriveConfigEndpoint("hd-drift-001"),
						httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
							Success: true,
							Result: newHyperdriveResponse("hd-drift-001", "tunnel-hyperdrive", apiHyperdriveOriginResponse{
								Host:     "db.internal.example.com",
								Port:     5432,
								Database: "mydb",
								User:     "dbuser",
								Scheme:   "postgresql",
							}),
						}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_hyperdrive_config.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestUnitHyperdriveConfig_MTLSDriftDetected(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result: func() apiHyperdriveResponse {
				resp := newHyperdriveResponse("hd-mtls-001", "mtls-hyperdrive", defaultHyperdriveOrigin)
				resp.MTLS = apiHyperdriveMTLSResponse{
					CACertificateID:   "ca-cert-1",
					MTLSCertificateID: "mtls-cert-1",
					Sslmode:           "verify-full",
				}
				return resp
			}(),
		}),
	)

	getMTLSResponder := func(mtls apiHyperdriveMTLSResponse) httpmock.Responder {
		resp := newHyperdriveResponse("hd-mtls-001", "mtls-hyperdrive", defaultHyperdriveOrigin)
		resp.MTLS = mtls
		return httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  resp,
		})
	}

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-mtls-001"),
		getMTLSResponder(apiHyperdriveMTLSResponse{
			CACertificateID:   "ca-cert-1",
			MTLSCertificateID: "mtls-cert-1",
			Sslmode:           "verify-full",
		}),
	)

	registerStandardDeleteResponder("hd-mtls-001")

	config := testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "mtls-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  mtls = {
    ca_certificate_id   = "ca-cert-1"
    mtls_certificate_id = "mtls-cert-1"
    sslmode             = "verify-full"
  }
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "mtls.ca_certificate_id", "ca-cert-1"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "mtls.mtls_certificate_id", "mtls-cert-1"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "mtls.sslmode", "verify-full"),
				),
			},
			{
				PreConfig: func() {
					// Simulate mtls being cleared out-of-band: the response no
					// longer includes any mtls fields.
					httpmock.RegisterResponder(http.MethodGet,
						hyperdriveConfigEndpoint("hd-mtls-001"),
						getMTLSResponder(apiHyperdriveMTLSResponse{}),
					)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("cloudflareext_hyperdrive_config.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

// TestUnitHyperdriveConfig_OmittedMTLSPreservesRemoteConfiguration reproduces
// the live API behavior investigated for issue #66. Omitting `mtls` from a
// PUT retains the remote mTLS configuration, although the immediate PUT
// response may not echo it. Removing the block from Terraform configuration
// should therefore leave mTLS unmanaged and omit it from subsequent PUTs.
// This test disables refresh and changes the live mTLS value out-of-band to
// prove Update does not overwrite it with the older value tracked in state.
func TestUnitHyperdriveConfig_OmittedMTLSPreservesRemoteConfiguration(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	currentMTLS := apiHyperdriveMTLSResponse{}
	currentName := "mtls-hyperdrive"
	var lastPutBody []byte

	respond := func(name string) apiHyperdriveResponse {
		resp := newHyperdriveResponse("hd-mtls-omit-001", name, defaultHyperdriveOrigin)
		resp.MTLS = currentMTLS
		return resp
	}
	applyWrite := func(bodyBytes []byte) (string, bool, error) {
		var body struct {
			Name string                     `json:"name"`
			MTLS *apiHyperdriveMTLSResponse `json:"mtls"`
		}
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			return "", false, err
		}
		if body.MTLS != nil {
			currentMTLS = *body.MTLS
		}
		currentName = body.Name
		return body.Name, body.MTLS != nil, nil
	}

	httpmock.RegisterResponder(http.MethodPost, hyperdriveConfigsEndpoint,
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			name, _, err := applyWrite(bodyBytes)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(name),
			})
		},
	)
	httpmock.RegisterResponder(http.MethodPut, hyperdriveConfigEndpoint("hd-mtls-omit-001"),
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			lastPutBody = bodyBytes
			name, mtlsIncluded, err := applyWrite(bodyBytes)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			result := respond(name)
			if !mtlsIncluded {
				// Match live API behavior: omission retains remote mTLS, but
				// the immediate PUT response does not echo the retained object.
				result.MTLS = apiHyperdriveMTLSResponse{}
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  result,
			})
		},
	)
	httpmock.RegisterResponder(http.MethodGet, hyperdriveConfigEndpoint("hd-mtls-omit-001"),
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(currentName),
			})
		},
	)
	registerStandardDeleteResponder("hd-mtls-omit-001")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		AdditionalCLIOptions: &resource.AdditionalCLIOptions{
			Plan: resource.PlanOptions{NoRefresh: true},
		},
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "mtls-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  mtls = {
    ca_certificate_id   = "ca-cert-1"
    mtls_certificate_id = "mtls-cert-1"
    sslmode             = "verify-full"
  }
}
`),
			},
			{
				PreConfig: func() {
					// Simulate a newer remote value that stale Terraform state
					// must not overwrite when refresh is disabled.
					currentMTLS = apiHyperdriveMTLSResponse{
						CACertificateID:   "ca-cert-remote",
						MTLSCertificateID: "mtls-cert-remote",
						Sslmode:           "verify-ca",
					}
				},
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "mtls-hyperdrive-renamed"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
			},
		},
	})

	if strings.Contains(string(lastPutBody), `"mtls"`) {
		t.Errorf("expected PUT body to omit unmanaged mtls, got: %s", lastPutBody)
	}
	if currentMTLS.CACertificateID != "ca-cert-remote" ||
		currentMTLS.MTLSCertificateID != "mtls-cert-remote" ||
		currentMTLS.Sslmode != "verify-ca" {
		t.Errorf("expected remote mtls to remain unchanged, got: %+v", currentMTLS)
	}
}

func TestUnitHyperdriveConfig_MTLSEmptyBlockNoDrift(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  newHyperdriveResponse("hd-mtls-002", "mtls-empty-hyperdrive", defaultHyperdriveOrigin),
		}),
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-mtls-002"),
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  newHyperdriveResponse("hd-mtls-002", "mtls-empty-hyperdrive", defaultHyperdriveOrigin),
		}),
	)

	registerStandardDeleteResponder("hd-mtls-002")

	config := testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "mtls-empty-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  mtls = {}
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
			},
			{
				// Refreshing (with a response that still omits mtls) must not
				// produce a diff: the config's empty `mtls = {}` block should
				// be preserved rather than nulled out.
				RefreshState:       true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestUnitHyperdriveConfig_APIError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
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
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				ExpectError: regexp.MustCompile(`403 Forbidden`),
			},
		},
	})
}

// setupHyperdriveCachingDisabledMock registers responders that mimic the real
// Cloudflare API behavior for caching-disabled configs (issue #58): any create
// or update request whose caching object carries `max_age` or
// `stale_while_revalidate` alongside `disabled: true` is rejected with error
// code 2007, and responses for a caching-disabled config omit those fields
// (represented here as zero values, which the SDK also produces for omitted
// fields).
func setupHyperdriveCachingDisabledMock(t *testing.T) {
	t.Helper()

	disabledCachingResult := func(name string) apiHyperdriveResponse {
		resp := newHyperdriveResponse("hd-test-id-001", name, defaultHyperdriveOrigin)
		resp.Caching = apiHyperdriveCachingResponse{Disabled: true}
		return resp
	}

	// current tracks the last written config so that GET reflects the actual
	// state instead of a fixed response (needed for multi-step tests).
	current := disabledCachingResult("my-hyperdrive")

	handleWrite := func(req *http.Request) (*http.Response, error) {
		var body struct {
			Name    string          `json:"name"`
			Caching map[string]any  `json:"caching"`
			Origin  json.RawMessage `json:"origin"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"code":400,"message":"invalid request"}]}`), nil
		}
		if disabled, _ := body.Caching["disabled"].(bool); disabled {
			if _, ok := body.Caching["max_age"]; ok {
				return httpmock.NewJsonResponse(400, shared.CloudflareResponse[any]{
					Success: false,
					Errors: []shared.CloudflareError{
						{Code: 2007, Message: "Invalid Hyperdrive config: caching: (max_age: caching must not be disabled in order to set max_age.)."},
					},
				})
			}
			if _, ok := body.Caching["stale_while_revalidate"]; ok {
				return httpmock.NewJsonResponse(400, shared.CloudflareResponse[any]{
					Success: false,
					Errors: []shared.CloudflareError{
						{Code: 2007, Message: "Invalid Hyperdrive config: caching: (stale_while_revalidate: caching must not be disabled in order to set stale_while_revalidate.)."},
					},
				})
			}
			current = disabledCachingResult(body.Name)
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  current,
			})
		}
		current = newHyperdriveResponse("hd-test-id-001", body.Name, defaultHyperdriveOrigin)
		return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  current,
		})
	}

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		handleWrite,
	)
	httpmock.RegisterResponder(http.MethodPut,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		handleWrite,
	)
	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  current,
			})
		},
	)
	registerStandardDeleteResponder("hd-test-id-001")
}

// TestUnitHyperdriveConfig_CachingDisabled reproduces issue #58: writing only
// `caching = { disabled = true }` must not send `max_age` /
// `stale_while_revalidate` to the API (the API rejects them with code 2007
// when caching is disabled), and the resulting state must leave both values
// null.
func TestUnitHyperdriveConfig_CachingDisabled(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveCachingDisabledMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = true
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "true"),
					resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age"),
					resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate"),
				),
			},
		},
	})
}

// TestUnitHyperdriveConfig_CachingEnabledToDisabled verifies the transition
// from enabled caching (with server defaults in state) to `disabled = true`:
// the update request must omit `max_age` / `stale_while_revalidate`, and both
// must become null in state instead of retaining the stale enabled-era values.
func TestUnitHyperdriveConfig_CachingEnabledToDisabled(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveCachingDisabledMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = false
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "false"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "60"),
				),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = true
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "true"),
					resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age"),
					resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate"),
				),
			},
			{
				// disabled -> enabled: the prior state values are null, so the
				// plan must carry unknown (not null) and accept the server-side
				// defaults (60/15) from the API response.
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = false
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "false"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "60"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate", "15"),
				),
			},
		},
	})
}

// TestUnitHyperdriveConfig_CachingDisabledConflictsWithMaxAge verifies that
// explicitly configuring `max_age` / `stale_while_revalidate` together with
// `disabled = true` is rejected at plan time with a clear error instead of
// failing on apply with the opaque Cloudflare error 2007.
func TestUnitHyperdriveConfig_CachingDisabledConflictsWithMaxAge(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = true
    max_age  = 30
  }
}
`),
				ExpectError: regexp.MustCompile(`(?s)max_age.+cannot be set.+disabled`),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled               = true
    stale_while_revalidate = 5
  }
}
`),
				ExpectError: regexp.MustCompile(`(?s)stale_while_revalidate.+cannot be set.+disabled`),
			},
		},
	})
}

// TestUnitHyperdriveConfig_CachingEmptyBlock verifies that an empty caching
// block (`caching = {}`) plans max_age / stale_while_revalidate as unknown
// (no static schema defaults) and accepts the Cloudflare server-side
// defaults (60/15) from the response.
func TestUnitHyperdriveConfig_CachingEmptyBlock(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {}
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "false"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "60"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate", "15"),
				),
			},
		},
	})
}

// TestUnitHyperdriveConfig_ImportNoCachingBlockNoDriftNoReset reproduces
// issue #64, live-verified against the Cloudflare API: importing a Hyperdrive
// config whose config omits the `caching` block (but whose remote caching is
// disabled) must populate `caching` into state on import (issue #60), *and*
// the subsequent plan for the caching-less config must not show a
// `caching = { disabled = true } -> null` diff. Applying that diff would send
// a full-replace PUT without `caching` and the API would silently reset
// remote caching to defaults.
//
// Step 2 asserts on the `caching.disabled` plan value specifically rather
// than requiring a literal empty plan (ExpectNonEmptyPlan: false). The first
// step is import-only (no prior apply), so origin.password_wo_version — a
// plain Optional trigger attribute, not itself write-only — is genuinely
// absent from the freshly-imported state (only Read-populated fields survive
// import) while the unchanged config still sets it to "1"; the resulting
// diff is real, expected write-only/import behavior (any post-import plan
// with a non-null password_wo_version shows it), and is unrelated to the
// caching regression under test here.
func TestUnitHyperdriveConfig_ImportNoCachingBlockNoDriftNoReset(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveCachingDisabledMock(t)

	config := testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "cloudflareext_hyperdrive_config.test",
				ImportState:        true,
				ImportStateId:      "hd-test-id-001",
				ImportStatePersist: true,
				Check: testutil.CheckResourceAttr(
					"cloudflareext_hyperdrive_config.test", "caching.disabled", "true",
				),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(
							"cloudflareext_hyperdrive_config.test",
							tfjsonpath.New("caching").AtMapKey("disabled"),
							knownvalue.Bool(true),
						),
					},
				},
			},
		},
	})
}

// TestUnitHyperdriveConfig_RemoveCachingBlockKeepsRemote encodes the #64 fix's
// "omitted = preserved" semantics directly: creating with an explicit
// `caching = { disabled = true }` block and then removing the block from
// config must not plan any change, since the remote caching configuration is
// left as-is when the block is omitted.
func TestUnitHyperdriveConfig_RemoveCachingBlockKeepsRemote(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveCachingDisabledMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    disabled = true
  }
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "true"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// hyperdriveCachingCaptureState tracks the last-written caching object (so
// GET reflects it, matching the real API) and the raw bytes of the last PUT
// request body, so a test can assert on exactly what was sent over the wire.
type hyperdriveCachingCaptureState struct {
	name        string
	current     apiHyperdriveCachingResponse
	lastPutBody []byte
}

// setupHyperdriveCachingCaptureMock registers stateful POST/PUT/GET/DELETE
// responders for hd-test-id-001: POST and PUT echo the request's `caching`
// object (when present) into a `current` value that GET subsequently
// returns, mimicking the real API's full-replace PUT semantics (a PUT that
// omits `caching` resets it to Cloudflare's defaults of 60/15). PUT also
// records its raw request body for the calling test to inspect.
func setupHyperdriveCachingCaptureMock(t *testing.T) *hyperdriveCachingCaptureState {
	t.Helper()

	state := &hyperdriveCachingCaptureState{
		name:    "my-hyperdrive",
		current: apiHyperdriveCachingResponse{Disabled: false, MaxAge: 60, StaleWhileRevalidate: 15},
	}

	respond := func(name string) apiHyperdriveResponse {
		resp := newHyperdriveResponse("hd-test-id-001", name, defaultHyperdriveOrigin)
		resp.Caching = state.current
		return resp
	}

	applyWrite := func(bodyBytes []byte) (string, error) {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(bodyBytes, &raw); err != nil {
			return "", err
		}
		var name string
		if err := json.Unmarshal(raw["name"], &name); err != nil {
			return "", err
		}
		if cachingRaw, ok := raw["caching"]; ok {
			var c apiHyperdriveCachingResponse
			if err := json.Unmarshal(cachingRaw, &c); err != nil {
				return "", err
			}
			state.current = c
		} else {
			// A full-replace request that omits `caching` resets it to the
			// Cloudflare API's server-side defaults.
			state.current = apiHyperdriveCachingResponse{Disabled: false, MaxAge: 60, StaleWhileRevalidate: 15}
		}
		state.name = name
		return name, nil
	}

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			name, err := applyWrite(bodyBytes)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(name),
			})
		},
	)

	httpmock.RegisterResponder(http.MethodPut,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			state.lastPutBody = bodyBytes
			name, err := applyWrite(bodyBytes)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(name),
			})
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(state.name),
			})
		},
	)

	registerStandardDeleteResponder("hd-test-id-001")

	return state
}

// TestUnitHyperdriveConfig_EmptyCachingBlockUnrelatedUpdateKeepsValues
// reproduces review finding 2: with `caching = {}` in config and an
// unrelated attribute (`name`) changing, the framework marks
// caching.max_age / caching.stale_while_revalidate unknown in the plan
// (Optional+Computed attributes without a plan modifier go unknown whenever
// any other attribute in the resource changes), so the request builder omits
// them from the full-replace PUT and the Cloudflare API resets them to its
// defaults (60/15) — even though `caching = {}` had already established
// max_age = 300 / stale_while_revalidate = 30. Adding
// int64planmodifier.UseStateForUnknown() as the first plan modifier on both
// attributes preserves the prior state value instead, so the PUT keeps
// carrying 300/30 regardless of what else in the resource changes.
func TestUnitHyperdriveConfig_EmptyCachingBlockUnrelatedUpdateKeepsValues(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	state := setupHyperdriveCachingCaptureMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {
    max_age                = 300
    stale_while_revalidate = 30
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "300"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate", "30"),
				),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive-renamed"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
  caching = {}
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "name", "my-hyperdrive-renamed"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "300"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate", "30"),
				),
			},
		},
	})

	if !strings.Contains(string(state.lastPutBody), `"max_age":300`) {
		t.Errorf("expected PUT body to contain max_age:300, got: %s", state.lastPutBody)
	}
	if !strings.Contains(string(state.lastPutBody), `"stale_while_revalidate":30`) {
		t.Errorf("expected PUT body to contain stale_while_revalidate:30, got: %s", state.lastPutBody)
	}
}

// TestUnitHyperdriveConfig_OmittedCachingBackfilledOnRefresh verifies the
// non-import backfill path: creating with the `caching` block omitted leaves
// `caching` null in state (Create's mapResponseToModel guard keeps applied
// state consistent with the null plan), a subsequent refresh backfills the
// API's caching object into state and produces an empty plan (config still
// omits `caching`, so preserveCachingStateWhenOmitted carries the freshly
// backfilled state value forward), and state is then populated with the
// values from the mocked GET response (disabled=false, max_age=60,
// stale_while_revalidate=15 from newHyperdriveResponse).
func TestUnitHyperdriveConfig_OmittedCachingBackfilledOnRefresh(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupHyperdriveMock()

	config := testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled"),
			},
			{
				RefreshState: true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config:   config,
				PlanOnly: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled", "false"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "60"),
					testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.stale_while_revalidate", "15"),
				),
			},
		},
	})
}

// hyperdriveRefreshSkippedFallbackState tracks the raw bytes of the last PUT
// request body (so a test can assert on exactly what was sent over the
// wire) and the marker caching object that GET always returns, mimicking
// caching having changed out-of-band since the last state refresh.
type hyperdriveRefreshSkippedFallbackState struct {
	name        string
	getCaching  apiHyperdriveCachingResponse
	lastPutBody []byte
}

// setupHyperdriveRefreshSkippedFallbackMock registers stateful POST/PUT/GET/DELETE
// responders for hd-test-id-001. Unlike setupHyperdriveCachingCaptureMock, GET
// always returns a fixed marker caching object regardless of what POST/PUT
// last wrote, so a test can distinguish values that could only have reached
// the outgoing PUT body via Update's pre-update fallback GET (see
// cachingParamFromResponse) from values that came from the plan/state.
func setupHyperdriveRefreshSkippedFallbackMock(t *testing.T, getCaching apiHyperdriveCachingResponse) *hyperdriveRefreshSkippedFallbackState {
	t.Helper()

	state := &hyperdriveRefreshSkippedFallbackState{
		name:       "my-hyperdrive",
		getCaching: getCaching,
	}

	respond := func(name string, caching apiHyperdriveCachingResponse) apiHyperdriveResponse {
		resp := newHyperdriveResponse("hd-test-id-001", name, defaultHyperdriveOrigin)
		resp.Caching = caching
		return resp
	}

	httpmock.RegisterResponder(http.MethodPost,
		hyperdriveConfigsEndpoint,
		func(req *http.Request) (*http.Response, error) {
			var body apiHyperdriveCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			state.name = body.Name
			// The Cloudflare API applies its server-side defaults
			// (60/15) when `caching` is omitted from the create request.
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(state.name, apiHyperdriveCachingResponse{Disabled: false, MaxAge: 60, StaleWhileRevalidate: 15}),
			})
		},
	)

	httpmock.RegisterResponder(http.MethodPut,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			state.lastPutBody = bodyBytes

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(bodyBytes, &raw); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			var name string
			if err := json.Unmarshal(raw["name"], &name); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false}`), nil
			}
			state.name = name

			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(state.name, state.getCaching),
			})
		},
	)

	httpmock.RegisterResponder(http.MethodGet,
		hyperdriveConfigEndpoint("hd-test-id-001"),
		func(_ *http.Request) (*http.Response, error) {
			// Always returns the marker caching object, as if it had
			// changed out-of-band since the resource was last refreshed.
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  respond(state.name, state.getCaching),
			})
		},
	)

	registerStandardDeleteResponder("hd-test-id-001")

	return state
}

// TestUnitHyperdriveConfig_RefreshSkippedUpdatePreservesRemoteCaching
// reproduces the scenario the Update fallback GET exists for: `caching` is
// omitted from config, so state's `caching` stays null (per
// preserveCachingStateWhenOmitted); if a plan is then produced without a
// refresh (e.g. `terraform apply -refresh=false` right after a fresh
// `terraform apply`, or any workflow where Terraform reuses stale state),
// Update sees a nil plan `data.Caching` and must fetch the live caching
// object via a GET before issuing the full-replace PUT, or the PUT would
// omit `caching` and the Cloudflare API would silently reset it to its
// defaults. The mocked GET returns a caching object {300, 30} that is
// distinct from both the API's create-time default (60/15, from POST) and
// anything in state/plan, so its presence in the captured PUT body can only
// be explained by the fallback GET having fired.
//
// AdditionalCLIOptions.Plan.NoRefresh (-> `-refresh=false`) is set at the
// TestCase level (terraform-plugin-testing v1.16.0's AdditionalCLIOptions
// field lives on TestCase, not TestStep) to suppress the refresh that
// terraform-plugin-testing would otherwise perform before every plan,
// which would otherwise backfill `caching` into state via Read/GET before
// Update ever saw a nil plan value.
func TestUnitHyperdriveConfig_RefreshSkippedUpdatePreservesRemoteCaching(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	state := setupHyperdriveRefreshSkippedFallbackMock(t, apiHyperdriveCachingResponse{
		Disabled:             false,
		MaxAge:               300,
		StaleWhileRevalidate: 30,
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		AdditionalCLIOptions: &resource.AdditionalCLIOptions{
			Plan: resource.PlanOptions{NoRefresh: true},
		},
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				Check: resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled"),
			},
			{
				// Only `name` changes; `caching` remains omitted from
				// config, and (with refresh suppressed for the whole
				// TestCase) state's `caching` is still null when this
				// step's plan is produced, so Update's fallback GET path
				// is exercised.
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive-renamed"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
			},
		},
	})

	if !strings.Contains(string(state.lastPutBody), `"max_age":300`) {
		t.Errorf("expected PUT body to contain max_age:300 (only obtainable via the fallback GET), got: %s", state.lastPutBody)
	}
	if !strings.Contains(string(state.lastPutBody), `"stale_while_revalidate":30`) {
		t.Errorf("expected PUT body to contain stale_while_revalidate:30 (only obtainable via the fallback GET), got: %s", state.lastPutBody)
	}
}

// TestUnitHyperdriveConfig_RefreshSkippedUpdatePreservesRemoteCachingZeroValues
// is a regression test for review finding 1: cachingParamFromResponse used
// to infer "the API omitted this field" from a zero Go value
// (c.MaxAge != 0), which would drop a legitimate zero from the fallback PUT.
// This drives the same fallback-GET path as
// TestUnitHyperdriveConfig_RefreshSkippedUpdatePreservesRemoteCaching, but
// the mocked GET returns max_age/stale_while_revalidate explicitly present
// in the JSON with value 0, and asserts the PUT body still carries `0`
// rather than silently omitting the field.
func TestUnitHyperdriveConfig_RefreshSkippedUpdatePreservesRemoteCachingZeroValues(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	state := setupHyperdriveRefreshSkippedFallbackMock(t, apiHyperdriveCachingResponse{
		Disabled:             false,
		MaxAge:               0,
		StaleWhileRevalidate: 0,
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		AdditionalCLIOptions: &resource.AdditionalCLIOptions{
			Plan: resource.PlanOptions{NoRefresh: true},
		},
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
				Check: resource.TestCheckNoResourceAttr("cloudflareext_hyperdrive_config.test", "caching.disabled"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive-renamed"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
			},
		},
	})

	if !strings.Contains(string(state.lastPutBody), `"max_age":0`) {
		t.Errorf("expected PUT body to contain the explicit max_age:0 from the fallback GET (not omit it), got: %s", state.lastPutBody)
	}
	if !strings.Contains(string(state.lastPutBody), `"stale_while_revalidate":0`) {
		t.Errorf("expected PUT body to contain the explicit stale_while_revalidate:0 from the fallback GET (not omit it), got: %s", state.lastPutBody)
	}
}

// TestUnitHyperdriveConfig_RefreshSkippedUpdateWithPopulatedStatePreservesRemoteCaching
// covers the refresh-skipped update path after caching has already been
// populated in state. Even then, an omitted caching block is unmanaged, so
// Update must preserve the current remote values rather than replaying stale
// state through the full-replace PUT.
func TestUnitHyperdriveConfig_RefreshSkippedUpdateWithPopulatedStatePreservesRemoteCaching(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	state := setupHyperdriveCachingCaptureMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		AdditionalCLIOptions: &resource.AdditionalCLIOptions{
			Plan: resource.PlanOptions{NoRefresh: true},
		},
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
			},
			{
				RefreshState: true,
				Check:        testutil.CheckResourceAttr("cloudflareext_hyperdrive_config.test", "caching.max_age", "60"),
			},
			{
				PreConfig: func() {
					state.current = apiHyperdriveCachingResponse{
						Disabled:             false,
						MaxAge:               300,
						StaleWhileRevalidate: 30,
					}
				},
				Config: testutil.TestConfig(`
resource "cloudflareext_hyperdrive_config" "test" {
  name = "my-hyperdrive-renamed"
  origin = {
    host     = "db.example.com"
    database = "mydb"
    user     = "dbuser"
    password_wo         = "dbpass"
    password_wo_version = "1"
  }
}
`),
			},
		},
	})

	if !strings.Contains(string(state.lastPutBody), `"max_age":300`) {
		t.Errorf("expected PUT body to preserve live max_age:300, got: %s", state.lastPutBody)
	}
	if !strings.Contains(string(state.lastPutBody), `"stale_while_revalidate":30`) {
		t.Errorf("expected PUT body to preserve live stale_while_revalidate:30, got: %s", state.lastPutBody)
	}
}
