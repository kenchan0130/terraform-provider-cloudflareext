package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResource = &SecretsStoreSecretEphemeral{}
var _ ephemeral.EphemeralResourceWithConfigure = &SecretsStoreSecretEphemeral{}

type SecretsStoreSecretEphemeral struct {
	client *CloudflareClient
}

// SecretsStoreSecretEphemeralModel is the ephemeral resource data model.
type SecretsStoreSecretEphemeralModel struct {
	StoreID  types.String `tfsdk:"store_id"`
	SecretID types.String `tfsdk:"secret_id"`
	Name     types.String `tfsdk:"name"`
	Status   types.String `tfsdk:"status"`
	Comment  types.String `tfsdk:"comment"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

func NewSecretsStoreSecretEphemeral() ephemeral.EphemeralResource {
	return &SecretsStoreSecretEphemeral{}
}

func (e *SecretsStoreSecretEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_store_secret"
}

func (e *SecretsStoreSecretEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Cloudflare Secrets Store secret metadata. " +
			"Ephemeral resources are never stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"store_id": schema.StringAttribute{
				Description: "The ID of the Secrets Store.",
				Required:    true,
			},
			"secret_id": schema.StringAttribute{
				Description: "The ID of the secret.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the secret.",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "The status of the secret.",
				Computed:    true,
			},
			"comment": schema.StringAttribute{
				Description: "A comment for the secret.",
				Computed:    true,
			},
			"created": schema.StringAttribute{
				Description: "The creation timestamp.",
				Computed:    true,
			},
			"modified": schema.StringAttribute{
				Description: "The last modification timestamp.",
				Computed:    true,
			},
		},
	}
}

func (e *SecretsStoreSecretEphemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*CloudflareClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *CloudflareClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	e.client = client
}

func (e *SecretsStoreSecretEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data SecretsStoreSecretEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		e.client.AccountID, data.StoreID.ValueString(), data.SecretID.ValueString())
	result, err := doRequest[apiSecretResponse](ctx, e.client, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read secret", err.Error())
		return
	}

	data.Name = types.StringValue(result.Name)
	data.Status = types.StringValue(result.Status)
	if result.Comment != "" {
		data.Comment = types.StringValue(result.Comment)
	} else {
		data.Comment = types.StringNull()
	}
	data.Created = types.StringValue(result.Created)
	data.Modified = types.StringValue(result.Modified)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
