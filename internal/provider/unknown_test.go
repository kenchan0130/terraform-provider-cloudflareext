package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/testutil"
)

// TestUnitProvider_UnknownAPIToken verifies that when the `api_token`
// attribute value is unknown at plan time (e.g. because it depends on
// another resource that hasn't been applied yet), the provider reports a
// clear "unknown value" error instead of misreporting it as a missing
// value (which would happen if Configure only checked IsNull()).
func TestUnitProvider_UnknownAPIToken(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `
resource "terraform_data" "test" {
  input = "test-token"
}

provider "cloudflareext" {
  api_token  = terraform_data.test.output
  account_id = "test-account-id"
  base_url   = "https://api.cloudflare.example.com/client/v4"
}

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
`,
				ExpectError: regexp.MustCompile(`Unknown Cloudflare API Token`),
			},
		},
	})
}
