package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testUnitTestProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"cloudflareext": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func testUnitTestConfig(extra string) string {
	return `
provider "cloudflareext" {
  api_token  = "test-api-token"
  account_id = "test-account-id"
  base_url   = "https://api.cloudflare.example.com/client/v4"
}
` + extra
}

func testCheckResourceAttr(name, key, value string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttr(name, key, value)
}
