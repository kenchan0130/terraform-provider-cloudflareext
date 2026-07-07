package provider

import (
	"context"
	"os"

	"github.com/cloudflare/cloudflare-go/v7/hyperdrive"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/secrets_store"
	"github.com/cloudflare/cloudflare-go/v7/workers"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
	hyperdriveresource "github.com/kenchan0130/terraform-provider-cloudflareext/internal/services/hyperdrive"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/services/secretsstore/secret"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/services/secretsstore/store"
	observabilitydestination "github.com/kenchan0130/terraform-provider-cloudflareext/internal/services/workers/observability/destination"
	workerssecret "github.com/kenchan0130/terraform-provider-cloudflareext/internal/services/workers/secret"
)

var _ provider.Provider = &CloudflareExtProvider{}

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

// New returns a new provider factory function.
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
					"Can also be set via the `CLOUDFLARE_API_TOKEN` environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"account_id": schema.StringAttribute{
				Description: "The Cloudflare account ID. " +
					"Can also be set via the `CLOUDFLARE_ACCOUNT_ID` environment variable.",
				Optional: true,
			},
			"base_url": schema.StringAttribute{
				Description: "The base URL for the Cloudflare API. " +
					"Defaults to `https://api.cloudflare.com/client/v4`. " +
					"Can also be set via the `CLOUDFLARE_API_BASE_URL` environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *CloudflareExtProvider) Configure(_ context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config CloudflareExtProviderModel

	resp.Diagnostics.Append(req.Config.Get(context.Background(), &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.APIToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Unknown Cloudflare API Token",
			"The provider cannot be configured because the `api_token` attribute value is unknown. "+
				"This usually means it depends on another resource that hasn't been applied yet. "+
				"Either apply that resource first, or set the value statically, "+
				"or set the `CLOUDFLARE_API_TOKEN` environment variable.",
		)
	}

	if config.AccountID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("account_id"),
			"Unknown Cloudflare Account ID",
			"The provider cannot be configured because the `account_id` attribute value is unknown. "+
				"This usually means it depends on another resource that hasn't been applied yet. "+
				"Either apply that resource first, or set the value statically, "+
				"or set the `CLOUDFLARE_ACCOUNT_ID` environment variable.",
		)
	}

	if config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Unknown Cloudflare API Base URL",
			"The provider cannot be configured because the `base_url` attribute value is unknown. "+
				"This usually means it depends on another resource that hasn't been applied yet. "+
				"Either apply that resource first, or set the value statically, "+
				"or set the `CLOUDFLARE_API_BASE_URL` environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	if !config.APIToken.IsNull() {
		apiToken = config.APIToken.ValueString()
	}
	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Cloudflare API Token",
			"The provider requires a Cloudflare API token. "+
				"Set the `api_token` attribute or the `CLOUDFLARE_API_TOKEN` environment variable.",
		)
		return
	}

	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if !config.AccountID.IsNull() {
		accountID = config.AccountID.ValueString()
	}
	if accountID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("account_id"),
			"Missing Cloudflare Account ID",
			"The provider requires a Cloudflare account ID. "+
				"Set the `account_id` attribute or the `CLOUDFLARE_ACCOUNT_ID` environment variable.",
		)
		return
	}

	baseURL := "https://api.cloudflare.com/client/v4"
	if envURL := os.Getenv("CLOUDFLARE_API_BASE_URL"); envURL != "" {
		baseURL = envURL
	}
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	opts := []option.RequestOption{
		option.WithAPIToken(apiToken),
		option.WithBaseURL(baseURL),
	}

	client := &shared.CloudflareClient{
		Hyperdrive:   hyperdrive.NewHyperdriveService(opts...),
		SecretsStore: secrets_store.NewSecretsStoreService(opts...),
		Workers:      workers.NewWorkerService(opts...),
		AccountID:    accountID,
		APIToken:     apiToken,
		BaseURL:      baseURL,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *CloudflareExtProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		hyperdriveresource.NewConfigResource,
		store.NewStoreResource,
		secret.NewSecretResource,
		workerssecret.NewSecretResource,
		observabilitydestination.NewDestinationResource,
	}
}

func (p *CloudflareExtProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		store.NewStoreDataSource,
		observabilitydestination.NewDestinationDataSource,
	}
}
