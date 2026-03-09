package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &CloudflareExtProvider{}
var _ provider.ProviderWithEphemeralResources = &CloudflareExtProvider{}

// CloudflareExtProvider is a minimal Terraform provider for Cloudflare resources
// that require write-only attribute support to prevent secrets from being stored in state.
type CloudflareExtProvider struct {
	version string
}

// CloudflareExtProviderModel describes the provider configuration data model.
type CloudflareExtProviderModel struct {
	APIToken  types.String `tfsdk:"api_token"`
	AccountID types.String `tfsdk:"account_id"`
	BaseURL   types.String `tfsdk:"base_url"`
}

// CloudflareClient holds the configuration for Cloudflare API calls.
type CloudflareClient struct {
	HTTPClient *http.Client
	BaseURL    string
	APIToken   string
	AccountID  string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CloudflareExtProvider{
			version: version,
		}
	}
}

func (p *CloudflareExtProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cloudflareext"
	resp.Version = p.version
}

func (p *CloudflareExtProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A minimal Cloudflare provider with write-only attribute support. " +
			"Manages Cloudflare resources that contain secrets (Hyperdrive, Secrets Store) " +
			"without persisting sensitive values in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Description: "The Cloudflare API token. " +
					"Can also be set via the CLOUDFLARE_API_TOKEN environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"account_id": schema.StringAttribute{
				Description: "The Cloudflare account ID. " +
					"Can also be set via the CLOUDFLARE_ACCOUNT_ID environment variable.",
				Optional: true,
			},
			"base_url": schema.StringAttribute{
				Description: "The base URL for the Cloudflare API. " +
					"Defaults to https://api.cloudflare.com/client/v4. " +
					"Can also be set via the CLOUDFLARE_API_BASE_URL environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *CloudflareExtProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config CloudflareExtProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// API token: config > environment variable
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	if !config.APIToken.IsNull() {
		apiToken = config.APIToken.ValueString()
	}
	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Cloudflare API Token",
			"The provider requires a Cloudflare API token. "+
				"Set the api_token attribute or the CLOUDFLARE_API_TOKEN environment variable.",
		)
		return
	}

	// Account ID: config > environment variable
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if !config.AccountID.IsNull() {
		accountID = config.AccountID.ValueString()
	}
	if accountID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("account_id"),
			"Missing Cloudflare Account ID",
			"The provider requires a Cloudflare account ID. "+
				"Set the account_id attribute or the CLOUDFLARE_ACCOUNT_ID environment variable.",
		)
		return
	}

	// Base URL: config > environment variable > default
	baseURL := "https://api.cloudflare.com/client/v4"
	if envURL := os.Getenv("CLOUDFLARE_API_BASE_URL"); envURL != "" {
		baseURL = envURL
	}
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	client := &CloudflareClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    baseURL,
		APIToken:   apiToken,
		AccountID:  accountID,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *CloudflareExtProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewHyperdriveConfigResource,
		NewSecretsStoreSecretResource,
	}
}

func (p *CloudflareExtProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *CloudflareExtProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewSecretsStoreSecretEphemeral,
	}
}
