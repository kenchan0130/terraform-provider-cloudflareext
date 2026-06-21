package destination

// normalizeLogpushDataset keeps provider state aligned with the values currently
// returned by Cloudflare's Workers Observability Destinations API. The public
// API documentation has used hyphenated values such as "opentelemetry-traces",
// but GET responses can return underscored values such as "opentelemetry_traces".
// Treat the response format as canonical here so imported/refreshed resources do
// not plan replacements solely because state was populated from the API.
func normalizeLogpushDataset(dataset string) string {
	switch dataset {
	case "opentelemetry-logs":
		return "opentelemetry_logs"
	case "opentelemetry-traces":
		return "opentelemetry_traces"
	case "opentelemetry-metrics":
		return "opentelemetry_metrics"
	default:
		return dataset
	}
}
