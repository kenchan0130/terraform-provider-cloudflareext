package hyperdrive_test

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

// apiHyperdriveResponse matches the Cloudflare Hyperdrive API response format.
// See: https://developers.cloudflare.com/api/resources/hyperdrive/subresources/configs/
type apiHyperdriveResponse struct {
	ID                    string                       `json:"id"`
	Name                  string                       `json:"name"`
	Origin                apiHyperdriveOriginResponse  `json:"origin"`
	Caching               apiHyperdriveCachingResponse `json:"caching"`
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
		httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
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
		httpmock.NewStringResponder(200, `{"success":true,"result":null}`),
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
				ExpectError: regexp.MustCompile(`Authentication error`),
			},
		},
	})
}
