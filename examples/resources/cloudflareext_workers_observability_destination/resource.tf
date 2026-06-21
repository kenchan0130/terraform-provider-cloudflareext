variable "otlp_api_key" {
  type      = string
  ephemeral = true
}

resource "cloudflareext_workers_observability_destination" "example" {
  name            = "workers-observability-example"
  enabled         = true
  type            = "logpush"
  url             = "https://otlp.example.com/v1/logs"
  logpush_dataset = "opentelemetry-logs"

  headers_wo = {
    Authorization = "Bearer ${var.otlp_api_key}"
  }
  headers_wo_version = "1"
}
