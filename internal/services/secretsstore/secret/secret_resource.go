package secret

import (
	"context"
	"fmt"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/secrets_store"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

var _ resource.Resource = &secretResource{}

type secretResource struct {
	client *shared.CloudflareClient
}

// NewSecretResource returns a new Secrets Store secret resource.
func NewSecretResource() resource.Resource {
	return &secretResource{}
}

func (r *secretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_store_secret"
}

func (r *secretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloudflare Secrets Store secret. " +
			"Supports write-only attributes to prevent secrets from being stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the secret.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"store_id": schema.StringAttribute{
				Description: "The ID of the Secrets Store.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the secret.",
				Required:    true,
			},
			"value": schema.StringAttribute{
				Description: "The secret value (legacy). " +
					"On Terraform 1.11+, use `value_wo` instead to prevent " +
					"the value from being stored in state. " +
					"Exactly one of `value` or `value_wo` must be set.",
				Optional:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("value_wo"),
					),
				},
			},
			"value_wo": schema.StringAttribute{
				Description: "The secret value (write-only). " +
					"This value is never stored in Terraform state. " +
					"Requires Terraform 1.11 or later. " +
					"Exactly one of `value` or `value_wo` must be set.",
				Optional:  true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("value"),
					),
				},
			},
			"value_wo_version": schema.StringAttribute{
				Description: "A version number that should be incremented each time `value_wo` changes. " +
					"Since `value_wo` is write-only and not stored in state, " +
					"Terraform cannot detect when it changes. " +
					"Incrementing this value triggers an update.",
				Optional: true,
			},
			"comment": schema.StringAttribute{
				Description: "A comment for the secret.",
				Optional:    true,
			},
			"scopes": schema.ListAttribute{
				Description: "The access scopes for the secret. Available: workers, ai_gateway, dex, access.",
				Required:    true,
				ElementType: types.StringType,
			},
			"status": schema.StringAttribute{
				Description: "The status of the secret.",
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

func (r *secretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*shared.CloudflareClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *shared.CloudflareClient, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *secretResource) resolveValue(data *model) string {
	if !data.ValueWO.IsNull() && !data.ValueWO.IsUnknown() {
		return data.ValueWO.ValueString()
	}
	return data.Value.ValueString()
}

func (r *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secretBody := secrets_store.StoreSecretNewParamsBody{
		Name:   cloudflare.F(data.Name.ValueString()),
		Value:  cloudflare.F(r.resolveValue(&data)),
		Scopes: cloudflare.F(scopes),
	}

	params := secrets_store.StoreSecretNewParams{
		AccountID: cloudflare.F(r.client.AccountID),
		Body:      []secrets_store.StoreSecretNewParamsBody{secretBody},
	}

	iter := r.client.Client.SecretsStore.Stores.Secrets.NewAutoPaging(ctx, data.StoreID.ValueString(), params)
	var secret *secrets_store.StoreSecretNewResponse
	for iter.Next() {
		item := iter.Current()
		secret = &item
		break
	}
	if err := iter.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to create secret", err.Error())
		return
	}

	if secret == nil {
		resp.Diagnostics.AddError("Failed to create secret", "API returned empty result")
		return
	}

	data.ID = types.StringValue(secret.ID)
	data.Name = types.StringValue(secret.Name)
	data.Status = types.StringValue(string(secret.Status))
	data.StoreID = types.StringValue(secret.StoreID)
	if secret.Comment != "" {
		data.Comment = types.StringValue(secret.Comment)
	}
	data.Created = types.StringValue(secret.Created.Format(time.RFC3339Nano))
	data.Modified = types.StringValue(secret.Modified.Format(time.RFC3339Nano))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Client.SecretsStore.Stores.Secrets.Get(ctx,
		data.StoreID.ValueString(),
		data.ID.ValueString(),
		secrets_store.StoreSecretGetParams{
			AccountID: cloudflare.F(r.client.AccountID),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read secret", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.Status = types.StringValue(string(result.Status))
	data.StoreID = types.StringValue(result.StoreID)
	if result.Comment != "" {
		data.Comment = types.StringValue(result.Comment)
	}
	data.Created = types.StringValue(result.Created.Format(time.RFC3339Nano))
	data.Modified = types.StringValue(result.Modified.Format(time.RFC3339Nano))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := secrets_store.StoreSecretEditParams{
		AccountID: cloudflare.F(r.client.AccountID),
		Name:      cloudflare.F(data.Name.ValueString()),
		Value:     cloudflare.F(r.resolveValue(&data)),
		Scopes:    cloudflare.F(scopes),
	}

	result, err := r.client.Client.SecretsStore.Stores.Secrets.Edit(ctx,
		data.StoreID.ValueString(),
		data.ID.ValueString(),
		params,
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update secret", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.Status = types.StringValue(string(result.Status))
	data.StoreID = types.StringValue(result.StoreID)
	if result.Comment != "" {
		data.Comment = types.StringValue(result.Comment)
	}
	data.Created = types.StringValue(result.Created.Format(time.RFC3339Nano))
	data.Modified = types.StringValue(result.Modified.Format(time.RFC3339Nano))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Client.SecretsStore.Stores.Secrets.Delete(ctx,
		data.StoreID.ValueString(),
		data.ID.ValueString(),
		secrets_store.StoreSecretDeleteParams{
			AccountID: cloudflare.F(r.client.AccountID),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete secret", err.Error())
		return
	}
}
