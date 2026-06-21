package destination

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
