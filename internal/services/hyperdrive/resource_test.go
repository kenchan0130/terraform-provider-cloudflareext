package hyperdrive_test

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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result: newHyperdriveResponse("hd-test-id-001", "my-hyperdrive", apiHyperdriveOriginResponse{
				Host:     "db.example.com",
				Port:     5432,
				Database: "mydb",
				User:     "dbuser",
				Scheme:   "postgresql",
			}),
		}),
	)

	httpmock.RegisterResponder(http.MethodPut,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
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

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)
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
							"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
							httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
								Success: true,
								Result: newHyperdriveResponse("hd-test-id-001", "my-hyperdrive-updated", apiHyperdriveOriginResponse{
									Host:     "db.example.com",
									Port:     5432,
									Database: "mydb",
									User:     "dbuser",
									Scheme:   "postgresql",
								}),
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
						"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
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
				ImportStateVerifyIgnore:              []string{"origin.password", "origin.password_wo", "origin.password_wo_version"},
			},
		},
	})
}

func TestUnitHyperdriveConfig_CustomPort(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-002",
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

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-002",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)

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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-access-001",
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

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-access-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)

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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-drift-001",
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

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-drift-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)

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
						"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-drift-001",
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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result: func() apiHyperdriveResponse {
				resp := newHyperdriveResponse("hd-mtls-001", "mtls-hyperdrive", apiHyperdriveOriginResponse{
					Host:     "db.example.com",
					Port:     5432,
					Database: "mydb",
					User:     "dbuser",
					Scheme:   "postgresql",
				})
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
		resp := newHyperdriveResponse("hd-mtls-001", "mtls-hyperdrive", apiHyperdriveOriginResponse{
			Host:     "db.example.com",
			Port:     5432,
			Database: "mydb",
			User:     "dbuser",
			Scheme:   "postgresql",
		})
		resp.MTLS = mtls
		return httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  resp,
		})
	}

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-mtls-001",
		getMTLSResponder(apiHyperdriveMTLSResponse{
			CACertificateID:   "ca-cert-1",
			MTLSCertificateID: "mtls-cert-1",
			Sslmode:           "verify-full",
		}),
	)

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-mtls-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)

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
						"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-mtls-001",
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

func TestUnitHyperdriveConfig_MTLSEmptyBlockNoDrift(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result: newHyperdriveResponse("hd-mtls-002", "mtls-empty-hyperdrive", apiHyperdriveOriginResponse{
				Host:     "db.example.com",
				Port:     5432,
				Database: "mydb",
				User:     "dbuser",
				Scheme:   "postgresql",
			}),
		}),
	)

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-mtls-002",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result: newHyperdriveResponse("hd-mtls-002", "mtls-empty-hyperdrive", apiHyperdriveOriginResponse{
				Host:     "db.example.com",
				Port:     5432,
				Database: "mydb",
				User:     "dbuser",
				Scheme:   "postgresql",
			}),
		}),
	)

	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-mtls-002",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)

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
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
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
		resp := newHyperdriveResponse("hd-test-id-001", name, apiHyperdriveOriginResponse{
			Host:     "db.example.com",
			Port:     5432,
			Database: "mydb",
			User:     "dbuser",
			Scheme:   "postgresql",
		})
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
		current = newHyperdriveResponse("hd-test-id-001", body.Name, apiHyperdriveOriginResponse{
			Host:     "db.example.com",
			Port:     5432,
			Database: "mydb",
			User:     "dbuser",
			Scheme:   "postgresql",
		})
		return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
			Success: true,
			Result:  current,
		})
	}

	httpmock.RegisterResponder(http.MethodPost,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs",
		handleWrite,
	)
	httpmock.RegisterResponder(http.MethodPut,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
		handleWrite,
	)
	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[apiHyperdriveResponse]{
				Success: true,
				Result:  current,
			})
		},
	)
	httpmock.RegisterResponder(http.MethodDelete,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/hyperdrive/configs/hd-test-id-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[any]{
			Success: true,
		}),
	)
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
