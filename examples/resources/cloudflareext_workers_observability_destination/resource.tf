resource "cloudflareext_workers_observability_destination" "grafana_traces" {
  name            = "grafana-traces"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp-gateway-prod-us-central-0.grafana.net/otlp/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers_wo = {
    Authorization = "Basic example"
  }

  headers_wo_version = "1"
}

resource "cloudflareext_workers_observability_destination" "grafana_traces_stateful_headers" {
  name            = "grafana-traces-stateful"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp-gateway-prod-us-central-0.grafana.net/otlp/v1/traces"
  logpush_dataset = "opentelemetry-traces"

  headers = {
    Authorization = "Basic example"
  }
}
