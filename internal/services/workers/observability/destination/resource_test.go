package destination_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jarcoal/httpmock"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

type testDestinationRequest struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Configuration struct {
		Headers        map[string]string `json:"headers"`
		LogpushDataset string            `json:"logpushDataset"`
		Type           string            `json:"type"`
		URL            string            `json:"url"`
	} `json:"configuration"`
}

type testDestinationResponse struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Scripts       []string `json:"scripts"`
	Configuration struct {
		Headers         map[string]string `json:"headers,omitempty"`
		DestinationConf string            `json:"destination_conf"`
		JobStatus       map[string]string `json:"jobStatus,omitempty"`
		LogpushDataset  string            `json:"logpushDataset"`
		LogpushJob      float64           `json:"logpushJob,omitempty"`
		Type            string            `json:"type"`
		URL             string            `json:"url"`
	} `json:"configuration"`
}

func newDestinationResponse(name string) testDestinationResponse {
	resp := testDestinationResponse{
		Slug:    "destination-001",
		Name:    name,
		Enabled: true,
		Scripts: []string{"worker-a"},
	}
	resp.Configuration.Headers = map[string]string{"Authorization": "Bearer test-token"}
	resp.Configuration.DestinationConf = "https://otlp.example.com/v1/logs"
	resp.Configuration.JobStatus = map[string]string{
		"error_message": "",
		"last_complete": "",
		"last_error":    "",
	}
	resp.Configuration.LogpushDataset = "opentelemetry-logs"
	resp.Configuration.LogpushJob = 123
	resp.Configuration.Type = "logpush"
	resp.Configuration.URL = "https://otlp.example.com/v1/logs"
	return resp
}

func setupDestinationMock(t *testing.T) {
	t.Helper()
	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"
	destination := newDestinationResponse("workers-observability-example")

	httpmock.RegisterResponder(http.MethodPost, baseURL,
		func(req *http.Request) (*http.Response, error) {
			var body testDestinationRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, ""), nil
			}
			if body.Configuration.Headers["Authorization"] != "Bearer test-token" {
				t.Fatalf("expected Authorization header in request, got %#v", body.Configuration.Headers)
			}
			destination.Name = body.Name
			destination.Enabled = body.Enabled
			destination.Configuration.LogpushDataset = body.Configuration.LogpushDataset
			destination.Configuration.Type = body.Configuration.Type
			destination.Configuration.URL = body.Configuration.URL
			return httpmock.NewJsonResponse(200, shared.CloudflareResponse[testDestinationResponse]{
				Success: true,
				Result:  destination,
			})
		},
	)

	httpmock.RegisterResponder(http.MethodGet, baseURL,
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[[]testDestinationResponse]{
			Success: true,
			Result:  []testDestinationResponse{destination},
		}),
	)

	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/destination-001",
		httpmock.NewJsonResponderOrPanic(200, shared.CloudflareResponse[testDestinationResponse]{
			Success: true,
			Result:  destination,
		}),
	)
}

func TestUnitWorkersObservabilityDestination_Create(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name               = "workers-observability-example"
  enabled            = true
  type               = "logpush"
  url                = "https://otlp.example.com/v1/logs"
  logpush_dataset    = "opentelemetry-logs"
  headers_wo         = { Authorization = "Bearer test-token" }
  headers_wo_version = "1"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "id", "destination-001"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "slug", "destination-001"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "name", "workers-observability-example"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "headers_wo_version", "1"),
					resource.TestCheckNoResourceAttr("cloudflareext_workers_observability_destination.test", "headers.%"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestinationDataSource_Read(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
data "cloudflareext_workers_observability_destination" "test" {
  name = "workers-observability-example"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "id", "destination-001"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "slug", "destination-001"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "logpush_dataset", "opentelemetry-logs"),
				),
			},
		},
	})
}
