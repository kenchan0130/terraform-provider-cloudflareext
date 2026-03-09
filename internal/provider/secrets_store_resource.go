package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SecretsStoreResource{}

type SecretsStoreResource struct {
	client *CloudflareClient
}

// SecretsStoreModel is the Terraform resource data model.
type SecretsStoreModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

type apiStoreCreateRequest struct {
	Name string `json:"name"`
}

type apiStoreResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

func NewSecretsStoreResource() resource.Resource {
	return &SecretsStoreResource{}
}

func (r *SecretsStoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_store"
}

func (r *SecretsStoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloudflare Secrets Store. " +
			"A store is a container for secrets that can be accessed by Cloudflare services.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the store.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the store. Cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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

func (r *SecretsStoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SecretsStoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SecretsStoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The Cloudflare API accepts an array of stores to create.
	apiReq := []apiStoreCreateRequest{
		{Name: data.Name.ValueString()},
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores", r.client.AccountID)
	result, err := doRequest[[]apiStoreResponse](ctx, r.client, http.MethodPost, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Secrets Store", err.Error())
		return
	}

	if len(*result) == 0 {
		resp.Diagnostics.AddError("Failed to create Secrets Store", "API returned empty result")
		return
	}

	store := (*result)[0]
	data.ID = types.StringValue(store.ID)
	data.Name = types.StringValue(store.Name)
	data.Created = types.StringValue(store.Created)
	data.Modified = types.StringValue(store.Modified)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretsStoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SecretsStoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	store, err := r.findStoreByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Secrets Store", err.Error())
		return
	}
	if store == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(store.ID)
	data.Name = types.StringValue(store.Name)
	data.Created = types.StringValue(store.Created)
	data.Modified = types.StringValue(store.Modified)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretsStoreResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No update API exists. All mutable attributes use RequiresReplace,
	// so this method should never be called.
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Secrets Store does not support updates. All attributes require replacement.",
	)
}

func (r *SecretsStoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SecretsStoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores/%s", r.client.AccountID, data.ID.ValueString())
	if err := doRequestNoBody(ctx, r.client, apiPath); err != nil {
		resp.Diagnostics.AddError("Failed to delete Secrets Store", err.Error())
		return
	}
}

func (r *SecretsStoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	store, err := r.findStoreByID(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import Secrets Store", err.Error())
		return
	}
	if store == nil {
		resp.Diagnostics.AddError("Failed to import Secrets Store", fmt.Sprintf("store with ID %q not found", req.ID))
		return
	}

	data := SecretsStoreModel{
		ID:       types.StringValue(store.ID),
		Name:     types.StringValue(store.Name),
		Created:  types.StringValue(store.Created),
		Modified: types.StringValue(store.Modified),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// findStoreByID lists all stores and finds one by ID.
// Returns nil if the store is not found.
func (r *SecretsStoreResource) findStoreByID(ctx context.Context, id string) (*apiStoreResponse, error) {
	apiPath := fmt.Sprintf("/accounts/%s/secrets_store/stores", r.client.AccountID)
	result, err := doRequest[[]apiStoreResponse](ctx, r.client, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, err
	}

	for _, store := range *result {
		if store.ID == id {
			return &store, nil
		}
	}

	return nil, nil
}
