package destination_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jarcoal/httpmock"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

func TestUnitWorkersObservabilityDestinationDataSource_Read(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations",
		destinationListResponder([]testDestinationResponse{
			newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", map[string]string{
				"Authorization": "Basic secret",
			}),
			newDestinationResponse("other-destination", "https://otlp.example.com/v1/other", nil),
		}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
data "cloudflareext_workers_observability_destination" "test" {
  name = "grafana-traces"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "id", "grafana-traces"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "slug", "grafana-traces"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "name", "grafana-traces"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "enabled", "true"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "type", "logpush"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "url", "https://otlp.example.com/v1/traces"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "logpush_dataset", "opentelemetry_traces"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "scripts.#", "1"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestinationDataSource_ReadSecondPage(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	baseURL := "https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations"
	httpmock.RegisterResponder(http.MethodGet,
		baseURL,
		destinationListResponderWithResultInfo(
			[]testDestinationResponse{
				newDestinationResponse("other-destination", "https://otlp.example.com/v1/other", nil),
			},
			&shared.ResultInfo{
				Page:       1,
				TotalPages: 2,
			},
		),
	)
	httpmock.RegisterResponder(http.MethodGet,
		baseURL+"?page=2",
		destinationListResponderWithResultInfo(
			[]testDestinationResponse{
				newDestinationResponse("grafana-traces", "https://otlp.example.com/v1/traces", nil),
			},
			&shared.ResultInfo{
				Page:       2,
				TotalPages: 2,
			},
		),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
data "cloudflareext_workers_observability_destination" "test" {
  name = "grafana-traces"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "id", "grafana-traces"),
					testutil.CheckResourceAttr("data.cloudflareext_workers_observability_destination.test", "name", "grafana-traces"),
				),
			},
		},
	})
}

func TestUnitWorkersObservabilityDestinationDataSource_NotFound(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet,
		"https://api.cloudflare.example.com/client/v4/accounts/test-account-id/workers/observability/destinations",
		destinationListResponder([]testDestinationResponse{}),
	)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testutil.TestConfig(`
data "cloudflareext_workers_observability_destination" "test" {
  name = "missing"
}
`),
				ExpectError: regexp.MustCompile(`Workers Observability Destination Not Found`),
			},
		},
	})
}
