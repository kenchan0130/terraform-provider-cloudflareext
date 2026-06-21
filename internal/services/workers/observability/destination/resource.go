package destination

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v7/workers"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
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
					stringvalidator.OneOf("opentelemetry-traces", "opentelemetry-logs", "opentelemetry-metrics"),
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
				PlanModifiers: []planmodifier.Float64{
					float64planmodifier.UseStateForUnknown(),
				},
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

func (r *destinationResource) createParams(ctx context.Context, data *model) (workers.ObservabilityDestinationNewParams, diag.Diagnostics) {
	headers, diags := r.resolveHeaders(ctx, data)
	configuration := workers.ObservabilityDestinationNewParamsConfiguration{}
	shared.SetParamField(&configuration.Headers, headers)
	shared.SetParamField(&configuration.LogpushDataset, workers.ObservabilityDestinationNewParamsConfigurationLogpushDataset(data.LogpushDataset.ValueString()))
	shared.SetParamField(&configuration.Type, workers.ObservabilityDestinationNewParamsConfigurationType(data.Type.ValueString()))
	shared.SetParamField(&configuration.URL, data.URL.ValueString())

	params := workers.ObservabilityDestinationNewParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	shared.SetParamField(&params.Configuration, configuration)
	shared.SetParamField(&params.Enabled, data.Enabled.ValueBool())
	shared.SetParamField(&params.Name, data.Name.ValueString())
	if !data.SkipPreflightCheck.IsNull() && !data.SkipPreflightCheck.IsUnknown() {
		shared.SetParamField(&params.SkipPreflightCheck, data.SkipPreflightCheck.ValueBool())
	}
	return params, diags
}

func (r *destinationResource) updateParams(ctx context.Context, data *model) (workers.ObservabilityDestinationUpdateParams, diag.Diagnostics) {
	headers, diags := r.resolveHeaders(ctx, data)
	configuration := workers.ObservabilityDestinationUpdateParamsConfiguration{}
	shared.SetParamField(&configuration.Headers, headers)
	shared.SetParamField(&configuration.Type, workers.ObservabilityDestinationUpdateParamsConfigurationType(data.Type.ValueString()))
	shared.SetParamField(&configuration.URL, data.URL.ValueString())

	params := workers.ObservabilityDestinationUpdateParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	shared.SetParamField(&params.Configuration, configuration)
	shared.SetParamField(&params.Enabled, data.Enabled.ValueBool())

	return params, diags
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
	if data.Headers.IsNull() || data.Headers.IsUnknown() {
		return nil
	}
	headersValue, diags := types.MapValueFrom(ctx, types.StringType, headers)
	if diags.HasError() {
		return diags
	}
	data.Headers = headersValue
	return diags
}

func applyNewResponse(ctx context.Context, data *model, result *workers.ObservabilityDestinationNewResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.ID = types.StringValue(result.Slug)
	data.Slug = types.StringValue(result.Slug)
	data.Name = types.StringValue(result.Name)
	data.Enabled = types.BoolValue(result.Enabled)
	data.Type = types.StringValue(string(result.Configuration.Type))
	data.URL = types.StringValue(result.Configuration.URL)
	data.LogpushDataset = types.StringValue(string(result.Configuration.LogpushDataset))
	data.DestinationConf = types.StringValue(result.Configuration.DestinationConf)
	data.LogpushJob = types.Float64Value(result.Configuration.LogpushJob)
	diags.Append(setScripts(ctx, data, result.Scripts)...)
	return diags
}

func applyUpdateResponse(ctx context.Context, data *model, result *workers.ObservabilityDestinationUpdateResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.ID = types.StringValue(result.Slug)
	data.Slug = types.StringValue(result.Slug)
	data.Name = types.StringValue(result.Name)
	data.Enabled = types.BoolValue(result.Enabled)
	data.Type = types.StringValue(string(result.Configuration.Type))
	data.URL = types.StringValue(result.Configuration.URL)
	data.DestinationConf = types.StringValue(result.Configuration.DestinationConf)
	data.LogpushJob = types.Float64Value(result.Configuration.LogpushJob)
	diags.Append(setScripts(ctx, data, result.Scripts)...)
	return diags
}

func applyListResponse(ctx context.Context, data *model, result *workers.ObservabilityDestinationListResponse) diag.Diagnostics {
	var diags diag.Diagnostics
	data.ID = types.StringValue(result.Slug)
	data.Slug = types.StringValue(result.Slug)
	data.Name = types.StringValue(result.Name)
	data.Enabled = types.BoolValue(result.Enabled)
	data.Type = types.StringValue(string(result.Configuration.Type))
	data.URL = types.StringValue(result.Configuration.URL)
	data.LogpushDataset = types.StringValue(string(result.Configuration.LogpushDataset))
	data.DestinationConf = types.StringValue(result.Configuration.DestinationConf)
	diags.Append(setScripts(ctx, data, result.Scripts)...)
	if !data.Headers.IsNull() {
		diags.Append(setHeaders(ctx, data, result.Configuration.Headers)...)
	}
	return diags
}

func (r *destinationResource) findDestinationBySlug(ctx context.Context, slug string) (*workers.ObservabilityDestinationListResponse, error) {
	params := workers.ObservabilityDestinationListParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	iter := r.client.Workers.Observability.Destinations.ListAutoPaging(ctx, params)
	for iter.Next() {
		destination := iter.Current()
		if destination.Slug == slug {
			return &destination, nil
		}
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to list destinations: %w", err)
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

	params, diags := r.createParams(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Workers.Observability.Destinations.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Workers Observability destination", err.Error())
		return
	}

	resp.Diagnostics.Append(applyNewResponse(ctx, &data, result)...)
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

	params, diags := r.updateParams(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Workers.Observability.Destinations.Update(ctx, data.Slug.ValueString(), params)
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

	params := workers.ObservabilityDestinationDeleteParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	_, err := r.client.Workers.Observability.Destinations.Delete(ctx, data.Slug.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Workers Observability destination", err.Error())
		return
	}
}

func (r *destinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("slug"), req.ID)...)
}
