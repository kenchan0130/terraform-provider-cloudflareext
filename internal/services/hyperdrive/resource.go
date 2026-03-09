package hyperdrive

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

var _ resource.Resource = &configResource{}
var _ resource.ResourceWithImportState = &configResource{}

type configResource struct {
	client *shared.CloudflareClient
}

// NewConfigResource returns a new Hyperdrive config resource.
func NewConfigResource() resource.Resource {
	return &configResource{}
}

func (r *configResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hyperdrive_config"
}

func (r *configResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloudflare Hyperdrive configuration. " +
			"Compatible with the official cloudflare_hyperdrive_config interface, " +
			"with an additional password_wo (write-only) attribute that prevents " +
			"the database password from being stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Hyperdrive configuration.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the Hyperdrive configuration.",
				Required:    true,
			},
			"origin": schema.SingleNestedAttribute{
				Description: "The database connection configuration.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						Description: "The hostname of the database server.",
						Required:    true,
					},
					"port": schema.Int64Attribute{
						Description: "The port number of the database server. Defaults to 5432.",
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(5432),
					},
					"database": schema.StringAttribute{
						Description: "The name of the database to connect to.",
						Required:    true,
					},
					"user": schema.StringAttribute{
						Description: "The username for database authentication.",
						Required:    true,
					},
					"password": schema.StringAttribute{
						Description: "The database password (legacy). " +
							"On Terraform 1.11+, use password_wo instead to prevent " +
							"the password from being stored in state. " +
							"Exactly one of password or password_wo must be set.",
						Optional:  true,
						Sensitive: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRoot("origin").AtName("password_wo"),
							),
						},
					},
					"password_wo": schema.StringAttribute{
						Description: "The database password (write-only). " +
							"This value is never stored in Terraform state. " +
							"Requires Terraform 1.11 or later. " +
							"Exactly one of password or password_wo must be set.",
						Optional:  true,
						WriteOnly: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRoot("origin").AtName("password"),
							),
						},
					},
					"scheme": schema.StringAttribute{
						Description: "The connection scheme. Defaults to \"postgresql\".",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("postgresql"),
					},
				},
			},
		},
	}
}

func (r *configResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *configResource) resolvePassword(origin *originModel) string {
	if !origin.PasswordWO.IsNull() && !origin.PasswordWO.IsUnknown() {
		return origin.PasswordWO.ValueString()
	}
	return origin.Password.ValueString()
}

func (r *configResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data configModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := apiCreateRequest{
		Name: data.Name.ValueString(),
		Origin: apiOrigin{
			Host:     data.Origin.Host.ValueString(),
			Port:     data.Origin.Port.ValueInt64(),
			Database: data.Origin.Database.ValueString(),
			User:     data.Origin.User.ValueString(),
			Password: r.resolvePassword(data.Origin),
			Scheme:   data.Origin.Scheme.ValueString(),
		},
	}

	apiPath := fmt.Sprintf("/accounts/%s/hyperdrive/configs", r.client.AccountID)
	result, err := shared.DoRequest[apiResponse](ctx, r.client, http.MethodPost, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Hyperdrive config", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *configResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/hyperdrive/configs/%s", r.client.AccountID, data.ID.ValueString())
	result, err := shared.DoRequest[apiResponse](ctx, r.client, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Hyperdrive config", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *configResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data configModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := apiCreateRequest{
		Name: data.Name.ValueString(),
		Origin: apiOrigin{
			Host:     data.Origin.Host.ValueString(),
			Port:     data.Origin.Port.ValueInt64(),
			Database: data.Origin.Database.ValueString(),
			User:     data.Origin.User.ValueString(),
			Password: r.resolvePassword(data.Origin),
			Scheme:   data.Origin.Scheme.ValueString(),
		},
	}

	apiPath := fmt.Sprintf("/accounts/%s/hyperdrive/configs/%s", r.client.AccountID, data.ID.ValueString())
	result, err := shared.DoRequest[apiResponse](ctx, r.client, http.MethodPut, apiPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Hyperdrive config", err.Error())
		return
	}

	r.mapResponseToModel(result, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *configResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/accounts/%s/hyperdrive/configs/%s", r.client.AccountID, data.ID.ValueString())
	if err := shared.DoRequestNoBody(ctx, r.client, apiPath); err != nil {
		resp.Diagnostics.AddError("Failed to delete Hyperdrive config", err.Error())
		return
	}
}

func (r *configResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *configResource) mapResponseToModel(result *apiResponse, data *configModel) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	if data.Origin == nil {
		data.Origin = &originModel{}
	}
	data.Origin.Host = types.StringValue(result.Origin.Host)
	data.Origin.Port = types.Int64Value(result.Origin.Port)
	data.Origin.Database = types.StringValue(result.Origin.Database)
	data.Origin.User = types.StringValue(result.Origin.User)
	data.Origin.Scheme = types.StringValue(result.Origin.Scheme)
}
