package provider

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
)

var _ resource.Resource = &SecretsStoreSecretResource{}

type SecretsStoreSecretResource struct {
	client *CloudflareClient
}

// SecretsStoreSecretModel is the Terraform resource data model.
type SecretsStoreSecretModel struct {
	ID       types.String `tfsdk:"id"`
	StoreID  types.String `tfsdk:"store_id"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	ValueWO  types.String `tfsdk:"value_wo"`
	Comment  types.String `tfsdk:"comment"`
	Scopes   types.List   `tfsdk:"scopes"`
	Status   types.String `tfsdk:"status"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

type apiSecretCreateRequest struct {
	Name    string   `json:"name"`
	Value   string   `json:"value"`
	Scopes  []string `json:"scopes"`
	Comment string   `json:"comment,omitempty"`
}

type apiSecretUpdateRequest struct {
	Name    string   `json:"name,omitempty"`
	Value   string   `json:"value,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

// apiSecretResponse is the Secrets Store API response.
// The value field is never returned in GET responses.
type apiSecretResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	StoreID  string `json:"store_id"`
	Comment  string `json:"comment"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

func NewSecretsStoreSecretResource() resource.Resource {
	return &SecretsStoreSecretResource{}
}

func (r *SecretsStoreSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_store_secret"
}

func (r *SecretsStoreSecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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

func (r *SecretsStoreSecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.client = client
}

// resolveValue returns the value from either value_wo or value.
func (r *SecretsStoreSecretResource) resolveValue(data *SecretsStoreSecretModel) string {
	if !data.ValueWO.IsNull() && !data.ValueWO.IsUnknown() {
		return data.ValueWO.ValueString()
	}
	return data.Value.ValueString()
}

func (r *SecretsStoreSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SecretsStoreSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := apiSecretCreateRequest{
		Name:    data.Name.ValueString(),
		Value:   r.resolveValue(&data),
		Scopes:  scopes,
		Comment: data.Comment.ValueString(),
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets", r.client.AccountID, data.StoreID.ValueString())
	result, err := doRequest[apiSecretResponse](ctx, r.client, http.MethodPost, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create secret", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretsStoreSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SecretsStoreSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		r.client.AccountID, data.StoreID.ValueString(), data.ID.ValueString())
	result, err := doRequest[apiSecretResponse](ctx, r.client, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read secret", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	// value / value_wo are never returned by the API

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretsStoreSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SecretsStoreSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := apiSecretUpdateRequest{
		Name:    data.Name.ValueString(),
		Value:   r.resolveValue(&data),
		Scopes:  scopes,
		Comment: data.Comment.ValueString(),
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		r.client.AccountID, data.StoreID.ValueString(), data.ID.ValueString())
	result, err := doRequest[apiSecretResponse](ctx, r.client, http.MethodPatch, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update secret", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretsStoreSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SecretsStoreSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s/secrets/%s",
		r.client.AccountID, data.StoreID.ValueString(), data.ID.ValueString())
	if err := doRequestNoBody(ctx, r.client, apiPath); err != nil {
		resp.Diagnostics.AddError("Failed to delete secret", err.Error())
		return
	}
}

func (r *SecretsStoreSecretResource) mapResponseToModel(result *apiSecretResponse, data *SecretsStoreSecretModel) {
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
