package secret

import (
	"context"
	"fmt"
	"net/http"

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
			"The secret value is never stored in Terraform state when using value_wo.",
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
					"On Terraform 1.11+, use value_wo instead to prevent " +
					"the value from being stored in state. " +
					"Exactly one of value or value_wo must be set.",
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
					"Exactly one of value or value_wo must be set.",
				Optional:  true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("value"),
					),
				},
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

	apiReq := apiCreateRequest{
		Name:    data.Name.ValueString(),
		Value:   r.resolveValue(&data),
		Scopes:  scopes,
		Comment: data.Comment.ValueString(),
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets", r.client.AccountID, data.StoreID.ValueString())
	result, err := shared.DoRequest[apiResponse](ctx, r.client, http.MethodPost, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create secret", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		r.client.AccountID, data.StoreID.ValueString(), data.ID.ValueString())
	result, err := shared.DoRequest[apiResponse](ctx, r.client, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read secret", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
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

	apiReq := apiUpdateRequest{
		Name:    data.Name.ValueString(),
		Value:   r.resolveValue(&data),
		Scopes:  scopes,
		Comment: data.Comment.ValueString(),
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		r.client.AccountID, data.StoreID.ValueString(), data.ID.ValueString())
	result, err := shared.DoRequest[apiResponse](ctx, r.client, http.MethodPatch, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update secret", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		r.client.AccountID, data.StoreID.ValueString(), data.ID.ValueString())
	if err := shared.DoRequestNoBody(ctx, r.client, apiPath); err != nil {
		resp.Diagnostics.AddError("Failed to delete secret", err.Error())
		return
	}
}

func (r *secretResource) mapResponseToModel(result *apiResponse, data *model) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.Status = types.StringValue(result.Status)
	data.StoreID = types.StringValue(result.StoreID)
	if result.Comment != "" {
		data.Comment = types.StringValue(result.Comment)
	}
	data.Created = types.StringValue(result.Created)
	data.Modified = types.StringValue(result.Modified)
}
