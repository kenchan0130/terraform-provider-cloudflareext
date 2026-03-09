package testutil

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider"
)

// ProtoV6ProviderFactories returns provider factories for unit testing.
func ProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"cloudflareext": providerserver.NewProtocol6WithError(provider.New("test")()),
	}
}

// TestConfig wraps HCL with the provider configuration block for unit testing.
func TestConfig(extra string) string {
	return `
provider "cloudflareext" {
  api_token  = "test-api-token"
  account_id = "test-account-id"
  base_url   = "https://api.cloudflare.example.com/client/v4"
}
` + extra
}

// CheckResourceAttr is an alias for resource.TestCheckResourceAttr.
func CheckResourceAttr(name, key, value string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttr(name, key, value)
}
