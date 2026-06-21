package destination

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

var _ datasource.DataSource = &destinationDataSource{}

type destinationDataSource struct {
	client *shared.CloudflareClient
}

type dataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Slug            types.String `tfsdk:"slug"`
	Name            types.String `tfsdk:"name"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Type            types.String `tfsdk:"type"`
	URL             types.String `tfsdk:"url"`
	LogpushDataset  types.String `tfsdk:"logpush_dataset"`
	Scripts         types.List   `tfsdk:"scripts"`
	DestinationConf types.String `tfsdk:"destination_conf"`
}

// NewDestinationDataSource returns a new Workers Observability destination data source.
func NewDestinationDataSource() datasource.DataSource {
	return &destinationDataSource{}
}

func (d *destinationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workers_observability_destination"
}

func (d *destinationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Cloudflare Workers Observability destination by name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the destination. This is the destination slug.",
				Computed:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The destination slug.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The destination name to look up.",
				Required:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the destination is enabled.",
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "The destination type.",
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("logpush"),
				},
			},
			"url": schema.StringAttribute{
				Description: "The OTLP endpoint URL.",
				Computed:    true,
			},
			"logpush_dataset": schema.StringAttribute{
				Description: "The OpenTelemetry dataset for this destination.",
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("opentelemetry_traces", "opentelemetry_logs", "opentelemetry_metrics"),
				},
			},
			"scripts": schema.ListAttribute{
				Description: "Workers scripts that reference this destination.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"destination_conf": schema.StringAttribute{
				Description: "The generated Logpush destination configuration.",
				Computed:    true,
			},
		},
	}
}

func (d *destinationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}

func (d *destinationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data dataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	destinations, err := listDestinations(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list Workers Observability destinations", err.Error())
		return
	}
	for _, destination := range destinations {
		if destination.Name != name {
			continue
		}

		data.ID = types.StringValue(destination.Slug)
		data.Slug = types.StringValue(destination.Slug)
		data.Name = types.StringValue(destination.Name)
		data.Enabled = types.BoolValue(destination.Enabled)
		data.Type = types.StringValue(destination.Configuration.Type)
		data.URL = types.StringValue(destination.Configuration.URL)
		data.LogpushDataset = types.StringValue(normalizeLogpushDataset(destination.Configuration.LogpushDataset))
		data.DestinationConf = types.StringValue(destination.Configuration.DestinationConf)

		scripts, diags := types.ListValueFrom(ctx, types.StringType, destination.Scripts)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Scripts = scripts

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	resp.Diagnostics.AddError(
		"Workers Observability Destination Not Found",
		fmt.Sprintf("No Workers Observability destination found with name %q.", name),
	)
}
