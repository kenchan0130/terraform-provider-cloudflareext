package store

import (
	"context"
	"fmt"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/secrets_store"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

var _ datasource.DataSource = &storeDataSource{}

type storeDataSource struct {
	client *shared.CloudflareClient
}

// storeDataSourceModel is the data source data model.
type storeDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

// NewStoreDataSource returns a new Secrets Store data source.
func NewStoreDataSource() datasource.DataSource {
	return &storeDataSource{}
}

func (d *storeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secrets_store"
}

func (d *storeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Cloudflare Secrets Store by name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the store.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the store to look up.",
				Required:    true,
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

func (d *storeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *storeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data storeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	iter := d.client.Client.SecretsStore.Stores.ListAutoPaging(ctx, secrets_store.StoreListParams{
		AccountID: cloudflare.F(d.client.AccountID),
	})
	for iter.Next() {
		store := iter.Current()
		if store.Name == name {
			data.ID = types.StringValue(store.ID)
			data.Name = types.StringValue(store.Name)
			data.Created = types.StringValue(store.Created.Format(time.RFC3339Nano))
			data.Modified = types.StringValue(store.Modified.Format(time.RFC3339Nano))

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	if err := iter.Err(); err != nil {
		resp.Diagnostics.AddError("Failed to list Secrets Stores", err.Error())
		return
	}

	resp.Diagnostics.AddError(
		"Secrets Store Not Found",
		fmt.Sprintf("No Secrets Store found with name %q.", name),
	)
}
