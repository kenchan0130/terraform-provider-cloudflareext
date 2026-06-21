package destination

import "strings"

// normalizeLogpushDataset keeps provider state aligned with the public
// hyphenated dataset values accepted by the schema. Cloudflare API responses can
// return underscored values such as "opentelemetry_traces"; normalize them to
// "opentelemetry-traces" to avoid replacement plans after import or refresh.
func normalizeLogpushDataset(dataset string) string {
	return strings.ReplaceAll(dataset, "_", "-")
}
