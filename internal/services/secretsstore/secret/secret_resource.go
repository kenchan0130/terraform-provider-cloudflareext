package secret

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/secrets_store"
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
					stringvalidator.AlsoRequires(
						path.MatchRoot("value_wo_version"),
					),
				},
			},
			"value_wo_version": schema.StringAttribute{
				Description: "A version number that should be incremented each time `value_wo` changes. " +
					"Since `value_wo` is write-only and not stored in state, " +
					"Terraform cannot detect when it changes. " +
					"Incrementing this value triggers an update. " +
					"Required when `value_wo` is set.",
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

func (r *secretResource) applyWriteOnlyAttributes(data, config *model) {
	data.ValueWO = config.ValueWO
}

// scopeKey normalizes a scope value so that hyphenated and underscored forms
// (e.g. "ai-gateway" vs "ai_gateway") are treated as equivalent. Cloudflare's
// API can echo back scopes in either form, which otherwise causes spurious
// "inconsistent result after apply" errors or permanent diffs.
func scopeKey(scope string) string {
	return strings.ReplaceAll(scope, "-", "_")
}

// reconcileScopes keeps provider state consistent with the plan when the
// Cloudflare API echoes back scopes that only differ in formatting or order.
//
// If the API scopes and the known/planned scopes are equal as a multiset after
// normalizing hyphen/underscore differences, the known scopes are returned
// unchanged, preserving the configuration's spelling and order. Otherwise
// (genuine remote drift) the API scopes are returned, though any element
// equivalent to a known scope still adopts the known scope's spelling.
func reconcileScopes(knownScopes, apiScopes []string) []string {
	remainingByKey := make(map[string][]string, len(knownScopes))
	for _, s := range knownScopes {
		key := scopeKey(s)
		remainingByKey[key] = append(remainingByKey[key], s)
	}

	if len(knownScopes) == len(apiScopes) {
		counts := make(map[string]int, len(remainingByKey))
		for key, queue := range remainingByKey {
			counts[key] = len(queue)
		}
		sameSet := true
		for _, s := range apiScopes {
			key := scopeKey(s)
			if counts[key] == 0 {
				sameSet = false
				break
			}
			counts[key]--
		}
		if sameSet {
			return knownScopes
		}
	}

	reconciled := make([]string, len(apiScopes))
	for i, s := range apiScopes {
		key := scopeKey(s)
		if queue := remainingByKey[key]; len(queue) > 0 {
			reconciled[i] = queue[0]
			remainingByKey[key] = queue[1:]
			continue
		}
		reconciled[i] = s
	}
	return reconciled
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

	var scopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secretBody := secrets_store.StoreSecretNewParamsBody{}
	shared.SetParamField(&secretBody.Name, data.Name.ValueString())
	shared.SetParamField(&secretBody.Value, r.resolveValue(&data))
	shared.SetParamField(&secretBody.Scopes, scopes)
	if !data.Comment.IsNull() && !data.Comment.IsUnknown() {
		shared.SetParamField(&secretBody.Comment, data.Comment.ValueString())
	}

	params := secrets_store.StoreSecretNewParams{
		Body: []secrets_store.StoreSecretNewParamsBody{secretBody},
	}
	shared.SetParamField(&params.AccountID, r.client.AccountID)

	iter := r.client.SecretsStore.Stores.Secrets.NewAutoPaging(ctx, data.StoreID.ValueString(), params)
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
	// If the API omits the comment and none was planned, keep it null so that
	// the stored state stays consistent with the plan.
	if secret.Comment != "" {
		data.Comment = types.StringValue(secret.Comment)
	} else if data.Comment.IsNull() || data.Comment.IsUnknown() {
		data.Comment = types.StringNull()
	}
	scopesList, scopesDiags := types.ListValueFrom(ctx, types.StringType, reconcileScopes(scopes, secret.Scopes))
	resp.Diagnostics.Append(scopesDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Scopes = scopesList
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

	result, err := r.client.SecretsStore.Stores.Secrets.Get(ctx,
		data.StoreID.ValueString(),
		data.ID.ValueString(),
		storeSecretGetParams(r.client.AccountID),
	)
	if err != nil {
		if shared.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read secret", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.Status = types.StringValue(string(result.Status))
	data.StoreID = types.StringValue(result.StoreID)
	// Reflect the remote comment as-is so drift (including removal) is detected.
	if result.Comment != "" {
		data.Comment = types.StringValue(result.Comment)
	} else {
		data.Comment = types.StringNull()
	}
	var priorScopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &priorScopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	scopesList, scopesDiags := types.ListValueFrom(ctx, types.StringType, reconcileScopes(priorScopes, result.Scopes))
	resp.Diagnostics.Append(scopesDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Scopes = scopesList
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

	// Write-only attributes are not available in the plan; read them from the config.
	var config model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	var scopes []string
	resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := secrets_store.StoreSecretEditParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	shared.SetParamField(&params.Value, r.resolveValue(&data))
	shared.SetParamField(&params.Scopes, scopes)
	if !data.Comment.IsNull() && !data.Comment.IsUnknown() {
		shared.SetParamField(&params.Comment, data.Comment.ValueString())
	} else {
		// The Secrets Store secret Edit API is a PATCH: omitted fields keep
		// their existing value. When the plan's comment is null (removed from
		// config), explicitly send an empty string to clear the remote
		// comment rather than leaving the stale value in place.
		shared.SetParamField(&params.Comment, "")
	}

	result, err := r.client.SecretsStore.Stores.Secrets.Edit(ctx,
		data.StoreID.ValueString(),
		data.ID.ValueString(),
		params,
		option.WithJSONSet("name", data.Name.ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update secret", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)
	data.Status = types.StringValue(string(result.Status))
	data.StoreID = types.StringValue(result.StoreID)
	// If the API omits the comment and none was planned, keep it null so that
	// the stored state stays consistent with the plan.
	if result.Comment != "" {
		data.Comment = types.StringValue(result.Comment)
	} else if data.Comment.IsNull() || data.Comment.IsUnknown() {
		data.Comment = types.StringNull()
	}
	scopesList, scopesDiags := types.ListValueFrom(ctx, types.StringType, reconcileScopes(scopes, result.Scopes))
	resp.Diagnostics.Append(scopesDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Scopes = scopesList
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

	_, err := r.client.SecretsStore.Stores.Secrets.Delete(ctx,
		data.StoreID.ValueString(),
		data.ID.ValueString(),
		storeSecretDeleteParams(r.client.AccountID),
	)
	if err != nil {
		if shared.IsNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete secret", err.Error())
		return
	}
}

func storeSecretGetParams(accountID string) secrets_store.StoreSecretGetParams {
	params := secrets_store.StoreSecretGetParams{}
	shared.SetParamField(&params.AccountID, accountID)
	return params
}

func storeSecretDeleteParams(accountID string) secrets_store.StoreSecretDeleteParams {
	params := secrets_store.StoreSecretDeleteParams{}
	shared.SetParamField(&params.AccountID, accountID)
	return params
}
