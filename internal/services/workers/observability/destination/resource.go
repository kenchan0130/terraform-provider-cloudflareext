package destination

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

var (
	_ resource.Resource                = &destinationResource{}
	_ resource.ResourceWithImportState = &destinationResource{}
)

type destinationResource struct {
	client *shared.CloudflareClient
}

// NewDestinationResource returns a new Workers Observability destination resource.
func NewDestinationResource() resource.Resource {
	return &destinationResource{}
}

func (r *destinationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workers_observability_destination"
}

func (r *destinationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloudflare Workers Observability destination.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the destination. This is the destination slug.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"slug": schema.StringAttribute{
				Description: "The destination slug.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The destination name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the destination is enabled.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The destination type. Currently only `logpush` is supported by Cloudflare.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("logpush"),
				},
			},
			"url": schema.StringAttribute{
				Description: "The OTLP endpoint URL.",
				Required:    true,
			},
			"logpush_dataset": schema.StringAttribute{
				Description: "The OpenTelemetry dataset for this destination.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("opentelemetry_traces", "opentelemetry_logs", "opentelemetry_metrics"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"headers": schema.MapAttribute{
				Description: "Custom headers sent to the OTLP endpoint (legacy). These values may contain authentication credentials and are stored in Terraform state. On Terraform 1.11+, use `headers_wo` instead.",
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.ConflictsWith(path.MatchRoot("headers_wo")),
					mapvalidator.PreferWriteOnlyAttribute(path.MatchRoot("headers_wo")),
				},
			},
			"headers_wo": schema.MapAttribute{
				Description: "Custom headers sent to the OTLP endpoint (write-only). These values are never stored in Terraform state. Requires Terraform 1.11 or later.",
				Optional:    true,
				WriteOnly:   true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.ConflictsWith(path.MatchRoot("headers")),
					mapvalidator.AlsoRequires(path.MatchRoot("headers_wo_version")),
				},
			},
			"headers_wo_version": schema.StringAttribute{
				Description: "A version number that should be incremented each time `headers_wo` changes. Since `headers_wo` is write-only and not stored in state, Terraform cannot detect when it changes.",
				Optional:    true,
			},
			"skip_preflight_check": schema.BoolAttribute{
				Description: "Whether to skip Cloudflare's destination preflight check during creation.",
				Optional:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"scripts": schema.ListAttribute{
				Description: "Workers scripts that reference this destination.",
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"destination_conf": schema.StringAttribute{
				Description: "The generated Logpush destination configuration.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"logpush_job": schema.Float64Attribute{
				Description: "The generated Logpush job identifier.",
				Computed:    true,
			},
		},
	}
}

func (r *destinationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *destinationResource) resolveHeaders(ctx context.Context, data *model) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	headers := map[string]string{}
	if !data.HeadersWO.IsNull() && !data.HeadersWO.IsUnknown() {
		diags.Append(data.HeadersWO.ElementsAs(ctx, &headers, false)...)
		return headers, diags
	}
	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		diags.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
	}
	return headers, diags
}

func destinationPath(client *shared.CloudflareClient) string {
	return fmt.Sprintf("accounts/%s/workers/observability/destinations", client.AccountID)
}

func destinationSlugPath(client *shared.CloudflareClient, slug string) string {
	return fmt.Sprintf("%s/%s", destinationPath(client), slug)
}

func (r *destinationResource) createBody(ctx context.Context, data *model) (apiDestinationRequest, diag.Diagnostics) {
	headers, diags := r.resolveHeaders(ctx, data)
	body := apiDestinationRequest{
		Configuration: apiDestinationRequestConfiguration{
			Headers:        headers,
			LogpushDataset: normalizeLogpushDataset(data.LogpushDataset.ValueString()),
			Type:           data.Type.ValueString(),
			URL:            data.URL.ValueString(),
		},
		Enabled: data.Enabled.ValueBool(),
		Name:    data.Name.ValueString(),
	}
	if !data.SkipPreflightCheck.IsNull() && !data.SkipPreflightCheck.IsUnknown() {
		skipPreflightCheck := data.SkipPreflightCheck.ValueBool()
		body.SkipPreflightCheck = &skipPreflightCheck
	}
	return body, diags
}

func (r *destinationResource) updateBody(ctx context.Context, data *model) (apiDestinationRequest, diag.Diagnostics) {
	headers, diags := r.resolveHeaders(ctx, data)
	return apiDestinationRequest{
		Configuration: apiDestinationRequestConfiguration{
			Headers: headers,
			Type:    data.Type.ValueString(),
			URL:     data.URL.ValueString(),
		},
		Enabled: data.Enabled.ValueBool(),
	}, diags
}

func (r *destinationResource) applyWriteOnlyAttributes(data, config *model) {
	data.HeadersWO = config.HeadersWO
}

func setScripts(ctx context.Context, data *model, scripts []string) diag.Diagnostics {
	scriptsValue, diags := types.ListValueFrom(ctx, types.StringType, scripts)
	if diags.HasError() {
		return diags
	}
	data.Scripts = scriptsValue
	return diags
}

func setHeaders(ctx context.Context, data *model, headers map[string]string) diag.Diagnostics {
	var diags diag.Diagnostics
	if data.Headers.IsNull() || data.Headers.IsUnknown() {
		return diags
	}
	headersValue, diags := types.MapValueFrom(ctx, types.StringType, headers)
	if diags.HasError() {
		return diags
	}
	data.Headers = headersValue
	return diags
}

func setLogpushJob(data *model, logpushJob *float64) {
	if logpushJob == nil {
		data.LogpushJob = types.Float64Null()
		return
	}
	data.LogpushJob = types.Float64Value(*logpushJob)
}

func setLogpushJobIfPresent(data *model, logpushJob *float64) {
	if logpushJob == nil {
		return
	}
	data.LogpushJob = types.Float64Value(*logpushJob)
}

func applyCreateResponse(ctx context.Context, data *model, result *apiDestinationResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.ID = types.StringValue(result.Slug)
	data.Slug = types.StringValue(result.Slug)
	data.Name = types.StringValue(result.Name)
	data.Enabled = types.BoolValue(result.Enabled)
	data.Type = types.StringValue(result.Configuration.Type)
	data.URL = types.StringValue(result.Configuration.URL)
	data.LogpushDataset = types.StringValue(normalizeLogpushDataset(result.Configuration.LogpushDataset))
	data.DestinationConf = types.StringValue(result.Configuration.DestinationConf)
	setLogpushJob(data, result.Configuration.LogpushJob)
	diags.Append(setScripts(ctx, data, result.Scripts)...)
	return diags
}

func applyUpdateResponse(ctx context.Context, data *model, result *apiDestinationResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.ID = types.StringValue(result.Slug)
	data.Slug = types.StringValue(result.Slug)
	data.Name = types.StringValue(result.Name)
	data.Enabled = types.BoolValue(result.Enabled)
	data.Type = types.StringValue(result.Configuration.Type)
	data.URL = types.StringValue(result.Configuration.URL)
	data.DestinationConf = types.StringValue(result.Configuration.DestinationConf)
	setLogpushJob(data, result.Configuration.LogpushJob)
	diags.Append(setScripts(ctx, data, result.Scripts)...)
	return diags
}

func applyListResponse(ctx context.Context, data *model, result *apiDestinationResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.ID = types.StringValue(result.Slug)
	data.Slug = types.StringValue(result.Slug)
	data.Name = types.StringValue(result.Name)
	data.Enabled = types.BoolValue(result.Enabled)
	data.Type = types.StringValue(result.Configuration.Type)
	data.URL = types.StringValue(result.Configuration.URL)
	data.LogpushDataset = types.StringValue(normalizeLogpushDataset(result.Configuration.LogpushDataset))
	data.DestinationConf = types.StringValue(result.Configuration.DestinationConf)
	setLogpushJobIfPresent(data, result.Configuration.LogpushJob)
	diags.Append(setScripts(ctx, data, result.Scripts)...)
	if !data.Headers.IsNull() {
		diags.Append(setHeaders(ctx, data, result.Configuration.Headers)...)
	}
	return diags
}

func listDestinations(ctx context.Context, client *shared.CloudflareClient) ([]apiDestinationResponse, error) {
	path := destinationPath(client)
	page := 1
	var all []apiDestinationResponse
	for {
		requestPath := path
		if page > 1 {
			requestPath = paginatedPath(path, page)
		}
		destinations, resultInfo, err := doRequestWithResultInfo[[]apiDestinationResponse](ctx, client, http.MethodGet, requestPath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list destinations: %w", err)
		}
		all = append(all, *destinations...)
		if resultInfo == nil || resultInfo.TotalPages == 0 {
			break
		}
		currentPage := page
		if resultInfo.Page > 0 {
			currentPage = resultInfo.Page
		}
		if currentPage >= resultInfo.TotalPages {
			break
		}
		page = currentPage + 1
	}
	return all, nil
}

func (r *destinationResource) findDestinationBySlug(ctx context.Context, slug string) (*apiDestinationResponse, error) {
	destinations, err := listDestinations(ctx, r.client)
	if err != nil {
		return nil, err
	}
	for _, destination := range destinations {
		if destination.Slug == slug {
			return &destination, nil
		}
	}
	return nil, nil
}

func (r *destinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	body, diags := r.createBody(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := doRequest[apiDestinationResponse](ctx, r.client, http.MethodPost, destinationPath(r.client), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Workers Observability destination", err.Error())
		return
	}

	resp.Diagnostics.Append(applyCreateResponse(ctx, &data, result)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *destinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	destination, err := r.findDestinationBySlug(ctx, data.Slug.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Workers Observability destination", err.Error())
		return
	}
	if destination == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(applyListResponse(ctx, &data, destination)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *destinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	body, diags := r.updateBody(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := doRequest[apiDestinationResponse](ctx, r.client, http.MethodPatch, destinationSlugPath(r.client, data.Slug.ValueString()), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Workers Observability destination", err.Error())
		return
	}

	resp.Diagnostics.Append(applyUpdateResponse(ctx, &data, result)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *destinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := doRequestNoBody(ctx, r.client, http.MethodDelete, destinationSlugPath(r.client, data.Slug.ValueString()), nil); err != nil {
		resp.Diagnostics.AddError("Failed to delete Workers Observability destination", err.Error())
		return
	}
}

func (r *destinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("slug"), req.ID)...)
}
