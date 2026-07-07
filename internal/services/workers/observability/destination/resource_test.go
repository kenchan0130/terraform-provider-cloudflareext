package destination_test

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

type testDestinationRequest struct {
	Configuration struct {
		Headers        map[string]string `json:"headers"`
		LogpushDataset string            `json:"logpushDataset"`
		Type           string            `json:"type"`
		URL            string            `json:"url"`
	} `json:"configuration"`
	Enabled            bool   `json:"enabled"`
	Name               string `json:"name,omitempty"`
	SkipPreflightCheck bool   `json:"skipPreflightCheck,omitempty"`
}

type testDestinationResponse struct {
	Configuration testDestinationConfiguration `json:"configuration"`
	Enabled       bool                         `json:"enabled"`
	Name          string                       `json:"name"`
	Scripts       []string                     `json:"scripts"`
	Slug          string                       `json:"slug"`
}

type testDestinationConfiguration struct {
	DestinationConf string            `json:"destination_conf"`
	Headers         map[string]string `json:"headers,omitempty"`
	JobStatus       map[string]string `json:"jobStatus,omitempty"`
	LogpushDataset  string            `json:"logpushDataset"`
	LogpushJob      *float64          `json:"logpushJob,omitempty"`
	Type            string            `json:"type"`
	URL             string            `json:"url"`
}

type testDestinationEnvelope[T any] struct {
	Errors     []map[string]string `json:"errors"`
	Messages   []map[string]string `json:"messages"`
	Result     T                   `json:"result"`
	ResultInfo *shared.ResultInfo  `json:"result_info,omitempty"`
	Success    bool                `json:"success"`
}

func newDestinationResponse(name, url string, headers map[string]string) testDestinationResponse {
	logpushJob := float64(123)
	return testDestinationResponse{
		Configuration: testDestinationConfiguration{
			DestinationConf: "https://api.cloudflare.com/client/v4/accounts/test-account-id/logpush/jobs/123",
			Headers:         headers,
			JobStatus:       map[string]string{},
			LogpushDataset:  "opentelemetry-traces",
			LogpushJob:      &logpushJob,
			Type:            "logpush",
			URL:             url,
		},
		Enabled: true,
		Name:    name,
		Scripts: []string{"worker-a"},
		Slug:    "grafana-traces",
	}
}

func withoutLogpushJob(destination testDestinationResponse) testDestinationResponse {
	destination.Configuration.LogpushJob = nil
	return destination
}

func destinationListResponder(destinations []testDestinationResponse) httpmock.Responder {
	return destinationListResponderWithResultInfo(destinations, nil)
}

func destinationListResponderWithResultInfo(destinations []testDestinationResponse, resultInfo *shared.ResultInfo) httpmock.Responder {
	return httpmock.NewJsonResponderOrPanic(200, testDestinationEnvelope[[]testDestinationResponse]{
		Errors:     []map[string]string{},
		Messages:   []map[string]string{},
		Result:     destinations,
		ResultInfo: resultInfo,
		Success:    true,
	})
}

func setupDestinationMock() {
	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"
	current := newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", map[string]string{
		"Authorization": "Basic secret",
	})

	httpmock.RegisterResponder(http.MethodPost, baseURL,
		func(req *http.Request) (*http.Response, error) {
			var body testDestinationRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"message":"invalid request"}]}`), nil
			}
			if body.Configuration.Headers["Authorization"] == "" {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"message":"missing authorization header"}]}`), nil
			}
			resp := newDestinationResponse(body.Name, body.Configuration.URL, nil)
			resp.Configuration.LogpushDataset = body.Configuration.LogpushDataset
			resp.Configuration.Type = body.Configuration.Type
			current = newDestinationResponse(body.Name, body.Configuration.URL, body.Configuration.Headers)
			current.Configuration.LogpushDataset = body.Configuration.LogpushDataset
			current.Configuration.Type = body.Configuration.Type
			return httpmock.NewJsonResponse(200, testDestinationEnvelope[testDestinationResponse]{
				Errors:   []map[string]string{},
				Messages: []map[string]string{{"message": "Resource created"}},
				Result:   resp,
				Success:  true,
			})
		},
	)

	httpmock.RegisterResponder(http.MethodGet, baseURL,
		func(_ *http.Request) (*http.Response, error) {
			return destinationListResponder([]testDestinationResponse{current})(nil)
		},
	)

	httpmock.RegisterResponder(http.MethodPatch, baseURL+"/grafana-traces",
		func(req *http.Request) (*http.Response, error) {
			var body testDestinationRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"message":"invalid request"}]}`), nil
			}
			resp := newDestinationResponse("grafana-traces", body.Configuration.URL, nil)
			resp.Configuration.Type = body.Configuration.Type
			current = newDestinationResponse("grafana-traces", body.Configuration.URL, body.Configuration.Headers)
			current.Configuration.Type = body.Configuration.Type
			return httpmock.NewJsonResponse(200, testDestinationEnvelope[testDestinationResponse]{
				Errors:   []map[string]string{},
				Messages: []map[string]string{{"message": "Resource updated"}},
				Result:   resp,
				Success:  true,
			})
		},
	)

	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/grafana-traces",
		httpmock.NewJsonResponderOrPanic(200, testDestinationEnvelope[testDestinationResponse]{
			Errors:   []map[string]string{},
			Messages: []map[string]string{{"message": "Resource deleted"}},
			Result:   newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", nil),
			Success:  true,
		}),
	)
}

func setupDestinationMockWithLogpushJobAfterUpdate() {
	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"
	current := withoutLogpushJob(newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", map[string]string{
		"Authorization": "Basic secret",
	}))

	httpmock.RegisterResponder(http.MethodPost, baseURL,
		func(req *http.Request) (*http.Response, error) {
			var body testDestinationRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"message":"invalid request"}]}`), nil
			}
			resp := withoutLogpushJob(newDestinationResponse(body.Name, body.Configuration.URL, nil))
			resp.Configuration.LogpushDataset = body.Configuration.LogpushDataset
			resp.Configuration.Type = body.Configuration.Type
			current = withoutLogpushJob(newDestinationResponse(body.Name, body.Configuration.URL, body.Configuration.Headers))
			current.Configuration.LogpushDataset = body.Configuration.LogpushDataset
			current.Configuration.Type = body.Configuration.Type
			return httpmock.NewJsonResponse(200, testDestinationEnvelope[testDestinationResponse]{
				Errors:   []map[string]string{},
				Messages: []map[string]string{{"message": "Resource created"}},
				Result:   resp,
				Success:  true,
			})
		},
	)

	httpmock.RegisterResponder(http.MethodGet, baseURL,
		func(_ *http.Request) (*http.Response, error) {
			return destinationListResponder([]testDestinationResponse{current})(nil)
		},
	)

	httpmock.RegisterResponder(http.MethodPatch, baseURL+"/grafana-traces",
		func(req *http.Request) (*http.Response, error) {
			var body testDestinationRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return httpmock.NewStringResponse(400, `{"success":false,"errors":[{"message":"invalid request"}]}`), nil
			}
			resp := newDestinationResponse("grafana-traces", body.Configuration.URL, nil)
			resp.Configuration.Type = body.Configuration.Type
			current = newDestinationResponse("grafana-traces", body.Configuration.URL, body.Configuration.Headers)
			current.Configuration.Type = body.Configuration.Type
			return httpmock.NewJsonResponse(200, testDestinationEnvelope[testDestinationResponse]{
				Errors:   []map[string]string{},
				Messages: []map[string]string{{"message": "Resource updated"}},
				Result:   resp,
				Success:  true,
			})
		},
	)

	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/grafana-traces",
		httpmock.NewJsonResponderOrPanic(200, testDestinationEnvelope[testDestinationResponse]{
			Errors:   []map[string]string{},
			Messages: []map[string]string{{"message": "Resource deleted"}},
			Result:   newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", nil),
			Success:  true,
		}),
	)
}

func TestUnitWorkersObservabilityDestination_CreateWithWriteOnlyHeaders(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic secret"
  }

  headers_wo_version = "1"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "id", "grafana-traces"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "slug", "grafana-traces"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "name", "grafana-traces"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "enabled", "true"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "type", "logpush"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "url", "https://otlp.example.com/v1/traces"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "logpush_dataset", "opentelemetry-traces"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "scripts.#", "1"),
					resource.TestCheckNoResourceAttr("cloudflareext_workers_observability_destination.test", "headers.%"),
					resource.TestCheckNoResourceAttr("cloudflareext_workers_observability_destination.test", "headers_wo.%"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_DeleteNotFoundSucceeds(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMock()

	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"

	// Simulate the destination already being deleted out-of-band: the delete
	// endpoint now returns 404. Destroy must still succeed.
	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/grafana-traces",
		httpmock.NewJsonResponderOrPanic(404, testDestinationEnvelope[json.RawMessage]{
			Errors:  []map[string]string{{"message": "destination not found"}},
			Success: false,
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic secret"
  }

  headers_wo_version = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "slug", "grafana-traces"),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_NormalizesLogpushDatasetToHyphen(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"
	current := newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", map[string]string{
		"Authorization": "Basic secret",
	})
	current.Configuration.LogpushDataset = "opentelemetry_traces"

	httpmock.RegisterResponder(http.MethodPost, baseURL,
		httpmock.NewJsonResponderOrPanic(200, testDestinationEnvelope[testDestinationResponse]{
			Errors:   []map[string]string{},
			Messages: []map[string]string{{"message": "Resource created"}},
			Result:   current,
			Success:  true,
		}),
	)
	httpmock.RegisterResponder(http.MethodGet, baseURL,
		func(_ *http.Request) (*http.Response, error) {
			return destinationListResponder([]testDestinationResponse{current})(nil)
		},
	)
	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/grafana-traces",
		httpmock.NewJsonResponderOrPanic(200, testDestinationEnvelope[testDestinationResponse]{
			Errors:   []map[string]string{},
			Messages: []map[string]string{{"message": "Resource deleted"}},
			Result:   current,
			Success:  true,
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic secret"
  }

  headers_wo_version = "1"
}
`),
				Check: testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "logpush_dataset", "opentelemetry-traces"),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_CreateWithStatefulHeaders(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers = {
    Authorization = "Basic secret"
  }
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "headers.%", "1"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "headers.Authorization", "Basic secret"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_ImportStateDoesNotReplaceLogpushDataset(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"
	current := newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", nil)
	current.Configuration.LogpushDataset = "opentelemetry_traces"

	httpmock.RegisterResponder(http.MethodGet, baseURL,
		func(_ *http.Request) (*http.Response, error) {
			return destinationListResponder([]testDestinationResponse{current})(nil)
		},
	)
	httpmock.RegisterResponder(http.MethodDelete, baseURL+"/grafana-traces",
		httpmock.NewJsonResponderOrPanic(200, testDestinationEnvelope[testDestinationResponse]{
			Errors:   []map[string]string{},
			Messages: []map[string]string{{"message": "Resource deleted"}},
			Result:   current,
			Success:  true,
		}),
	)

	config := testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"
}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "cloudflareext_workers_observability_destination.test",
				ImportState:        true,
				ImportStateId:      "grafana-traces",
				ImportStatePersist: true,
				Check: testutil.CheckResourceAttr(
					"cloudflareext_workers_observability_destination.test",
					"logpush_dataset",
					"opentelemetry-traces",
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_UpdateCanSetPreviouslyNullLogpushJob(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMockWithLogpushJobAfterUpdate()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic secret"
  }

  headers_wo_version = "1"
}
`),
				Check: resource.TestCheckNoResourceAttr("cloudflareext_workers_observability_destination.test", "logpush_job"),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces-updated"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic new-secret"
  }

  headers_wo_version = "2"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "url", "https://otlp.example.com/v1/traces-updated"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "logpush_job", "123"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_Update(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	setupDestinationMock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic secret"
  }

  headers_wo_version = "1"
}
`),
			},
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces-updated"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic new-secret"
  }

  headers_wo_version = "2"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "url", "https://otlp.example.com/v1/traces-updated"),
					testutil.CheckResourceAttr("cloudflareext_workers_observability_destination.test", "headers_wo_version", "2"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestination_WriteOnlyHeadersRequireVersion(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
resource "cloudflareext_workers_observability_destination" "test" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic secret"
  }
}
`),
				ExpectError: regexp.MustCompile(`headers_wo_version`),
			},
		},
	})
}
