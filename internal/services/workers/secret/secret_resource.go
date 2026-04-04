package secret

import (
	"context"
	"fmt"
	"strings"

	cloudflare "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/workers"
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

var (
	_ resource.Resource                = &secretResource{}
	_ resource.ResourceWithImportState = &secretResource{}
)

type secretResource struct {
	client *shared.CloudflareClient
}

// NewSecretResource returns a new Workers Script secret resource.
func NewSecretResource() resource.Resource {
	return &secretResource{}
}

func (r *secretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workers_script_secret"
}

func (r *secretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloudflare Workers Script secret. " +
			"Supports write-only attributes to prevent secrets from being stored in Terraform state.",
		Attributes: map[string]schema.Attribute{
			"script_name": schema.StringAttribute{
				Description: "The name of the Workers script.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "A JavaScript variable name for the secret binding.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"text": schema.StringAttribute{
				Description: "The secret value (legacy). " +
					"On Terraform 1.11+, use `text_wo` instead to prevent " +
					"the value from being stored in state. " +
					"Exactly one of `text` or `text_wo` must be set.",
				Optional:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("text_wo"),
					),
				},
			},
			"text_wo": schema.StringAttribute{
				Description: "The secret value (write-only). " +
					"This value is never stored in Terraform state. " +
					"Requires Terraform 1.11 or later. " +
					"Exactly one of `text` or `text_wo` must be set.",
				Optional:  true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("text"),
					),
				},
			},
			"text_wo_version": schema.StringAttribute{
				Description: "A version number that should be incremented each time `text_wo` changes. " +
					"Since `text_wo` is write-only and not stored in state, " +
					"Terraform cannot detect when it changes. " +
					"Incrementing this value triggers an update.",
				Optional: true,
			},
			"type": schema.StringAttribute{
				Description: "The kind of resource that the binding provides.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

func (r *secretResource) resolveText(data *model) string {
	if !data.TextWO.IsNull() && !data.TextWO.IsUnknown() {
		return data.TextWO.ValueString()
	}
	return data.Text.ValueString()
}

func (r *secretResource) applyWriteOnlyAttributes(data, config *model) {
	data.TextWO = config.TextWO
}

func (r *secretResource) upsert(ctx context.Context, data *model, operation string) error {
	params := workers.ScriptSecretUpdateParams{
		AccountID: cloudflare.F(r.client.AccountID),
		Body: workers.ScriptSecretUpdateParamsBodyWorkersBindingKindSecretText{
			Name: cloudflare.F(data.Name.ValueString()),
			Text: cloudflare.F(r.resolveText(data)),
			Type: cloudflare.F(workers.ScriptSecretUpdateParamsBodyWorkersBindingKindSecretTextTypeSecretText),
		},
	}

	result, err := r.client.Client.Workers.Scripts.Secrets.Update(ctx, data.ScriptName.ValueString(), params)
	if err != nil {
		return fmt.Errorf("failed to %s Workers script secret: %w", operation, err)
	}

	data.Name = types.StringValue(result.Name)
	data.Type = types.StringValue(string(result.Type))
	return nil
}

func (r *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are not available in the plan; read them from the config.
	var config model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	if err := r.upsert(ctx, &data, "create"); err != nil {
		resp.Diagnostics.AddError("Failed to create Workers script secret", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Client.Workers.Scripts.Secrets.Get(ctx,
		data.ScriptName.ValueString(),
		data.Name.ValueString(),
		workers.ScriptSecretGetParams{
			AccountID: cloudflare.F(r.client.AccountID),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Workers script secret", err.Error())
		return
	}

	data.Name = types.StringValue(result.Name)
	data.Type = types.StringValue(string(result.Type))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are not available in the plan; read them from the config.
	var config model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	if err := r.upsert(ctx, &data, "update"); err != nil {
		resp.Diagnostics.AddError("Failed to update Workers script secret", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Client.Workers.Scripts.Secrets.Delete(ctx,
		data.ScriptName.ValueString(),
		data.Name.ValueString(),
		workers.ScriptSecretDeleteParams{
			AccountID: cloudflare.F(r.client.AccountID),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Workers script secret", err.Error())
		return
	}
}

func (r *secretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`Expected format: "script_name/secret_name"`,
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("script_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
}
