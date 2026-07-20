package hyperdrive

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v7/hyperdrive"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kenchan0130/terraform-provider-cloudflareext/internal/provider/shared"
)

var _ resource.Resource = &configResource{}
var _ resource.ResourceWithImportState = &configResource{}
var _ resource.ResourceWithValidateConfig = &configResource{}

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
			"Compatible with the official `cloudflare_hyperdrive_config` interface, " +
			"with additional write-only attributes (`password_wo`, `access_client_secret_wo`) " +
			"that prevent secrets from being stored in Terraform state.",
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
			"origin":                  originSchemaAttribute(),
			"caching":                 cachingSchemaAttribute(),
			"mtls":                    mtlsSchemaAttribute(),
			"origin_connection_limit": originConnectionLimitSchemaAttribute(),
		},
	}
}

func originSchemaAttribute() schema.Attribute {
	return schema.SingleNestedAttribute{
		Description: "The database connection configuration.",
		Required:    true,
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "The hostname of the database server.",
				Required:    true,
			},
			"port": schema.Int64Attribute{
				Description: "The port number of the database server. Defaults to `5432`.",
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
					"On Terraform 1.11+, use `password_wo` instead to prevent " +
					"the password from being stored in state. " +
					"Exactly one of `password` or `password_wo` must be set.",
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
					"Exactly one of `password` or `password_wo` must be set.",
				Optional:  true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("origin").AtName("password"),
					),
					stringvalidator.AlsoRequires(
						path.MatchRoot("origin").AtName("password_wo_version"),
					),
				},
			},
			"password_wo_version": schema.StringAttribute{
				Description: "A version number that should be incremented each time `password_wo` changes. " +
					"Since `password_wo` is write-only and not stored in state, " +
					"Terraform cannot detect when it changes. " +
					"Incrementing this value triggers an update. " +
					"Required when `password_wo` is set.",
				Optional: true,
			},
			"scheme": schema.StringAttribute{
				Description: "The connection scheme. Valid values: `postgresql`, `postgres`, `mysql`. " +
					"Defaults to `\"postgresql\"`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("postgresql"),
				Validators: []validator.String{
					stringvalidator.OneOf("postgresql", "postgres", "mysql"),
				},
			},
			"access_client_id": schema.StringAttribute{
				Description: "The Client ID of the Access token to use when connecting to the origin database.",
				Optional:    true,
			},
			"access_client_secret": schema.StringAttribute{
				Description: "The Client Secret of the Access token (legacy). " +
					"On Terraform 1.11+, use `access_client_secret_wo` instead. " +
					"At most one of `access_client_secret` or `access_client_secret_wo` may be set.",
				Optional:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("origin").AtName("access_client_secret_wo"),
					),
				},
			},
			"access_client_secret_wo": schema.StringAttribute{
				Description: "The Client Secret of the Access token (write-only). " +
					"This value is never stored in Terraform state. " +
					"Requires Terraform 1.11 or later. " +
					"At most one of `access_client_secret` or `access_client_secret_wo` may be set.",
				Optional:  true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("origin").AtName("access_client_secret"),
					),
					stringvalidator.AlsoRequires(
						path.MatchRoot("origin").AtName("access_client_secret_wo_version"),
					),
				},
			},
			"access_client_secret_wo_version": schema.StringAttribute{
				Description: "A version number that should be incremented each time `access_client_secret_wo` changes. " +
					"Since `access_client_secret_wo` is write-only and not stored in state, " +
					"Terraform cannot detect when it changes. " +
					"Incrementing this value triggers an update. " +
					"Required when `access_client_secret_wo` is set.",
				Optional: true,
			},
		},
	}
}

func cachingSchemaAttribute() schema.Attribute {
	return schema.SingleNestedAttribute{
		Description: "The caching configuration for the Hyperdrive. " +
			"When this block is omitted, the remote caching configuration is left " +
			"unmanaged: it is tracked in state and preserved on updates. " +
			"An empty `caching = {}` block re-enables caching if it was disabled " +
			"but preserves the current max_age / stale_while_revalidate values; " +
			"to reset those to Cloudflare's defaults, set them explicitly " +
			"(max_age = 60, stale_while_revalidate = 15).",
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			preserveCachingStateWhenOmitted{},
		},
		Attributes: map[string]schema.Attribute{
			"disabled": schema.BoolAttribute{
				Description: "Whether to disable caching of SQL responses. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"max_age": schema.Int64Attribute{
				Description: "The maximum duration (in seconds) for which items should persist in the cache. " +
					"When caching is enabled and this is not set, the Cloudflare API applies its default of `60`. " +
					"Maximum value is `3600`. Cannot be set when `disabled` is `true` " +
					"(the API rejects it with error 2007).",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					// UseStateForUnknown must run first: without it, config
					// omitting caching's children while an unrelated attribute
					// changes elsewhere in the resource marks these Computed
					// attributes unknown in the plan (documented framework
					// behavior), the request builder then omits them from the
					// full-replace PUT, and the API resets them to defaults.
					// Preserving the prior state value keeps `caching = {}`
					// drift-free regardless of what else in the resource
					// changes. nullWhenCachingDisabledModifier still runs after
					// to override to null/unknown on a disabled transition.
					int64planmodifier.UseStateForUnknown(),
					nullWhenCachingDisabledModifier{},
				},
			},
			"stale_while_revalidate": schema.Int64Attribute{
				Description: "The number of seconds the cache may serve a stale response while revalidating. " +
					"When caching is enabled and this is not set, the Cloudflare API applies its default of `15`. " +
					"Cannot be set when `disabled` is `true` (the API rejects it with error 2007).",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					nullWhenCachingDisabledModifier{},
				},
			},
		},
	}
}

// preserveCachingStateWhenOmitted keeps the `caching` object out of the plan
// diff when config omits it, carrying forward the prior state value instead.
//
// The framework marks a Computed attribute unknown in the plan when config is
// null. An unknown *object* cannot be decoded into the pointer-based
// `*cachingModel` (data.Caching would need a non-nil zero value with unknown
// scalar fields, which the framework does not produce for a wholly-unknown
// object), and unknown would also cause spurious "(known after apply)" churn
// on every plan. Preserving the prior state value instead keeps the diff
// empty and ensures the full-replace PUT continues to carry the prior
// caching values, so the remote configuration is left as-is (issue #64).
//
// On create (no prior state), there is nothing to preserve, so the plan is
// left null; Create's mapResponseToModel guard (`data.Caching != nil`) keeps
// applied state consistent with that null plan, and the first Read/Refresh
// afterward backfills the API's caching object into state.
type preserveCachingStateWhenOmitted struct{}

func (m preserveCachingStateWhenOmitted) Description(_ context.Context) string {
	return "Preserves the prior state value when caching is omitted from config, instead of planning unknown."
}

func (m preserveCachingStateWhenOmitted) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m preserveCachingStateWhenOmitted) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.IsNull() {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.ObjectNull(req.PlanValue.AttributeTypes(ctx))
}

// nullWhenCachingDisabledModifier forces caching.max_age /
// caching.stale_while_revalidate to null in the plan when
// caching.disabled is true and the attribute is not set in config.
//
// Without this, the Optional+Computed attributes would carry their prior
// state values (from a caching-enabled era) into the plan on an
// enabled -> disabled transition, the request builder would send them, and
// the Cloudflare API would reject the config with error 2007
// ("caching must not be disabled in order to set max_age"). Disabling
// caching also discards these settings server-side, so null is the only
// state value that stays drift-free (issue #58).
type nullWhenCachingDisabledModifier struct{}

func (m nullWhenCachingDisabledModifier) Description(_ context.Context) string {
	return "Forces the value to null when caching.disabled is true and the attribute is not configured."
}

func (m nullWhenCachingDisabledModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m nullWhenCachingDisabledModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// Explicitly configured values are left as-is; the disabled+value
	// combination is rejected by ValidateConfig (and re-checked at apply
	// time) with a clear error.
	if !req.ConfigValue.IsNull() {
		return
	}

	var disabled types.Bool
	diags := req.Plan.GetAttribute(ctx, path.Root("caching").AtName("disabled"), &disabled)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// disabled is unknown at plan time (e.g. derived from another
	// resource): the outcome may be either null (disabled) or a
	// server-provided default (enabled), so the only plan value consistent
	// with both is unknown. Leaving the state-preserved value here would
	// make the applied state contradict the plan.
	if disabled.IsUnknown() {
		resp.PlanValue = types.Int64Unknown()
		return
	}
	if disabled.IsNull() {
		return
	}

	if disabled.ValueBool() {
		resp.PlanValue = types.Int64Null()
		return
	}

	// disabled is false. On a disabled -> enabled transition the prior
	// state value is null; planning null would contradict the applied
	// state once the API fills in its server-side default (60/15), so
	// plan unknown instead.
	if req.PlanValue.IsNull() {
		resp.PlanValue = types.Int64Unknown()
	}
}

func mtlsSchemaAttribute() schema.Attribute {
	return schema.SingleNestedAttribute{
		Description: "The mTLS configuration for connecting to the origin database.",
		Optional:    true,
		Attributes: map[string]schema.Attribute{
			"ca_certificate_id": schema.StringAttribute{
				Description: "The UUID of a custom CA certificate to use when connecting to the origin database.",
				Optional:    true,
			},
			"mtls_certificate_id": schema.StringAttribute{
				Description: "The UUID of a custom mTLS client certificate to use when connecting to the origin database.",
				Optional:    true,
			},
			"sslmode": schema.StringAttribute{
				Description: "The SSL mode to use when connecting to the origin database. " +
					"Valid values: `require`, `verify-ca`, `verify-full`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("require", "verify-ca", "verify-full"),
				},
			},
		},
	}
}

func originConnectionLimitSchemaAttribute() schema.Attribute {
	return schema.Int64Attribute{
		Description: "The soft maximum number of connections that Hyperdrive may establish to the origin database. " +
			"Must be between `5` and `100`.",
		Optional: true,
		Computed: true,
		// Without a plan modifier, the framework marks every null-in-config
		// Computed attribute of a resource as unknown whenever *any* other
		// attribute in that resource requires an update (this is documented
		// framework behavior, not specific to this attribute). That produced
		// a spurious "(known after apply)" for origin_connection_limit on
		// every unrelated update (e.g. removing the `caching` block per issue
		// #64), which is exactly the class of bug UseStateForUnknown exists
		// to prevent for Optional+Computed attributes without a static
		// default (the `id` attribute above uses the same pattern).
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
		Validators: []validator.Int64{
			int64validator.Between(5, 100),
		},
	}
}

// ValidateConfig rejects caching.max_age / caching.stale_while_revalidate
// when caching.disabled is true. The Cloudflare API rejects this combination
// with error 2007 ("caching must not be disabled in order to set max_age"),
// so failing at plan time gives a clear error instead of an opaque apply
// failure (issue #58).
func (r *configResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data configModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Caching == nil {
		return
	}
	if !cachingExplicitlyDisabled(data.Caching) {
		return
	}
	// Unknown values are skipped here (they may still resolve to null);
	// the combination is re-checked at apply time in Create/Update via
	// validateCachingCombination once all values are resolved.
	appendCachingCombinationErrors(data.Caching, &resp.Diagnostics)
}

// cachingExplicitlyDisabled reports whether caching.disabled is known and true.
func cachingExplicitlyDisabled(caching *cachingModel) bool {
	return !caching.Disabled.IsNull() && !caching.Disabled.IsUnknown() && caching.Disabled.ValueBool()
}

// appendCachingCombinationErrors adds an attribute error for each of
// caching.max_age / caching.stale_while_revalidate that is a known non-null
// value while caching.disabled is true.
func appendCachingCombinationErrors(caching *cachingModel, diags *diag.Diagnostics) {
	if !caching.MaxAge.IsNull() && !caching.MaxAge.IsUnknown() {
		diags.AddAttributeError(
			path.Root("caching").AtName("max_age"),
			"Invalid Attribute Combination",
			"caching.max_age cannot be set when caching.disabled is true. "+
				"The Cloudflare API rejects this combination, and disabling caching discards caching settings server-side.",
		)
	}
	if !caching.StaleWhileRevalidate.IsNull() && !caching.StaleWhileRevalidate.IsUnknown() {
		diags.AddAttributeError(
			path.Root("caching").AtName("stale_while_revalidate"),
			"Invalid Attribute Combination",
			"caching.stale_while_revalidate cannot be set when caching.disabled is true. "+
				"The Cloudflare API rejects this combination, and disabling caching discards caching settings server-side.",
		)
	}
}

// validateCachingCombination re-checks the disabled + max_age /
// stale_while_revalidate combination at apply time with resolved config
// values. ValidateConfig must skip unknown values (they may resolve to
// null), so without this re-check an explicitly configured value that only
// becomes known at apply time would be silently dropped from the request
// while remaining in the plan — producing an inconsistent result.
func validateCachingCombination(plan, config *configModel, diags *diag.Diagnostics) {
	if plan.Caching == nil || config.Caching == nil {
		return
	}
	if !cachingExplicitlyDisabled(plan.Caching) {
		return
	}
	appendCachingCombinationErrors(config.Caching, diags)
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

func (r *configResource) applyWriteOnlyAttributes(data, config *configModel) {
	if config.Origin != nil && data.Origin != nil {
		data.Origin.PasswordWO = config.Origin.PasswordWO
		data.Origin.AccessClientSecretWO = config.Origin.AccessClientSecretWO
	}
}

func (r *configResource) resolvePassword(origin *originModel) string {
	if !origin.PasswordWO.IsNull() && !origin.PasswordWO.IsUnknown() {
		return origin.PasswordWO.ValueString()
	}
	return origin.Password.ValueString()
}

func (r *configResource) resolveAccessClientSecret(origin *originModel) string {
	if !origin.AccessClientSecretWO.IsNull() && !origin.AccessClientSecretWO.IsUnknown() {
		return origin.AccessClientSecretWO.ValueString()
	}
	if !origin.AccessClientSecret.IsNull() && !origin.AccessClientSecret.IsUnknown() {
		return origin.AccessClientSecret.ValueString()
	}
	return ""
}

func (r *configResource) buildSDKParams(data *configModel) hyperdrive.HyperdriveParam {
	var origin hyperdrive.HyperdriveOriginUnionParam
	if !data.Origin.AccessClientID.IsNull() && !data.Origin.AccessClientID.IsUnknown() {
		originParam := hyperdrive.HyperdriveOriginAccessProtectedDatabaseBehindCloudflareTunnelParam{}
		shared.SetParamField(&originParam.Host, data.Origin.Host.ValueString())
		shared.SetParamField(&originParam.Database, data.Origin.Database.ValueString())
		shared.SetParamField(&originParam.User, data.Origin.User.ValueString())
		shared.SetParamField(&originParam.Password, r.resolvePassword(data.Origin))
		shared.SetParamField(&originParam.Scheme, hyperdrive.HyperdriveOriginAccessProtectedDatabaseBehindCloudflareTunnelScheme(data.Origin.Scheme.ValueString()))
		shared.SetParamField(&originParam.AccessClientID, data.Origin.AccessClientID.ValueString())
		shared.SetParamField(&originParam.AccessClientSecret, r.resolveAccessClientSecret(data.Origin))
		origin = originParam
	} else {
		originParam := hyperdrive.HyperdriveOriginPublicDatabaseParam{}
		shared.SetParamField(&originParam.Host, data.Origin.Host.ValueString())
		shared.SetParamField(&originParam.Port, data.Origin.Port.ValueInt64())
		shared.SetParamField(&originParam.Database, data.Origin.Database.ValueString())
		shared.SetParamField(&originParam.User, data.Origin.User.ValueString())
		shared.SetParamField(&originParam.Password, r.resolvePassword(data.Origin))
		shared.SetParamField(&originParam.Scheme, hyperdrive.HyperdriveOriginPublicDatabaseScheme(data.Origin.Scheme.ValueString()))
		origin = originParam
	}

	params := hyperdrive.HyperdriveParam{}
	shared.SetParamField(&params.Name, data.Name.ValueString())
	shared.SetParamField(&params.Origin, origin)

	if data.Caching != nil {
		shared.SetParamField(&params.Caching, buildCachingParam(data.Caching))
	}

	if data.MTLS != nil {
		mtlsParam := hyperdrive.HyperdriveMTLSParam{}
		if !data.MTLS.CACertificateID.IsNull() && !data.MTLS.CACertificateID.IsUnknown() {
			shared.SetParamField(&mtlsParam.CACertificateID, data.MTLS.CACertificateID.ValueString())
		}
		if !data.MTLS.MTLSCertificateID.IsNull() && !data.MTLS.MTLSCertificateID.IsUnknown() {
			shared.SetParamField(&mtlsParam.MTLSCertificateID, data.MTLS.MTLSCertificateID.ValueString())
		}
		if !data.MTLS.SSLMode.IsNull() && !data.MTLS.SSLMode.IsUnknown() {
			shared.SetParamField(&mtlsParam.Sslmode, data.MTLS.SSLMode.ValueString())
		}
		shared.SetParamField(&params.MTLS, mtlsParam)
	}

	if !data.OriginConnectionLimit.IsNull() && !data.OriginConnectionLimit.IsUnknown() {
		shared.SetParamField(&params.OriginConnectionLimit, data.OriginConnectionLimit.ValueInt64())
	}

	return params
}

// buildCachingParam maps the caching model to the SDK param. The API rejects
// max_age / stale_while_revalidate when disabled is true (error 2007), so
// they are omitted from the request in that case (issue #58).
func buildCachingParam(caching *cachingModel) hyperdrive.HyperdriveCachingHyperdriveHyperdriveCachingEnabledParam {
	cachingParam := hyperdrive.HyperdriveCachingHyperdriveHyperdriveCachingEnabledParam{}
	disabledKnown := !caching.Disabled.IsNull() && !caching.Disabled.IsUnknown()
	if disabledKnown {
		shared.SetParamField(&cachingParam.Disabled, caching.Disabled.ValueBool())
	}
	if disabledKnown && caching.Disabled.ValueBool() {
		return cachingParam
	}
	if !caching.MaxAge.IsNull() && !caching.MaxAge.IsUnknown() {
		shared.SetParamField(&cachingParam.MaxAge, caching.MaxAge.ValueInt64())
	}
	if !caching.StaleWhileRevalidate.IsNull() && !caching.StaleWhileRevalidate.IsUnknown() {
		shared.SetParamField(&cachingParam.StaleWhileRevalidate, caching.StaleWhileRevalidate.ValueInt64())
	}
	return cachingParam
}

// cachingParamFromResponse builds a caching SDK param from a Get response,
// for the Update fallback that fetches remote caching when the plan's
// caching is nil (see Update). It mirrors buildCachingParam's omit-when-disabled
// semantics by routing through a synthesized cachingModel, so the
// "don't send max_age/stale_while_revalidate when disabled" logic isn't
// duplicated: Disabled is always set; when true, MaxAge/StaleWhileRevalidate
// are left null (buildCachingParam omits them regardless); when false, they
// are set based on JSON presence (c.JSON.MaxAge.IsMissing() /
// c.JSON.StaleWhileRevalidate.IsMissing()), not on whether the value is
// zero, so a legitimate zero value from the API is preserved instead of
// being dropped from the fallback PUT.
func cachingParamFromResponse(c *hyperdrive.HyperdriveCaching) hyperdrive.HyperdriveCachingHyperdriveHyperdriveCachingEnabledParam {
	model := cachingModel{
		Disabled: types.BoolValue(c.Disabled),
	}
	if !c.Disabled && !c.JSON.MaxAge.IsMissing() {
		model.MaxAge = types.Int64Value(c.MaxAge)
	} else {
		model.MaxAge = types.Int64Null()
	}
	if !c.Disabled && !c.JSON.StaleWhileRevalidate.IsMissing() {
		model.StaleWhileRevalidate = types.Int64Value(c.StaleWhileRevalidate)
	} else {
		model.StaleWhileRevalidate = types.Int64Null()
	}
	return buildCachingParam(&model)
}

func (r *configResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data configModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are not available in the plan; read them from the config.
	var config configModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	validateCachingCombination(&data, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	params := hyperdrive.ConfigNewParams{
		Hyperdrive: r.buildSDKParams(&data),
	}
	shared.SetParamField(&params.AccountID, r.client.AccountID)

	result, err := r.client.Hyperdrive.Configs.New(ctx, params)
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

	params := hyperdrive.ConfigGetParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	result, err := r.client.Hyperdrive.Configs.Get(ctx, data.ID.ValueString(), params)
	if err != nil {
		if shared.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read Hyperdrive config", err.Error())
		return
	}

	// Read always reflects the API's caching object into state: the API
	// always returns one, but prior state may lack it (e.g. right after
	// import, or for resources created before caching became Computed).
	// Create/Update keep the plan-driven `data.Caching != nil` guard in
	// mapResponseToModel so applied state never contradicts a planned null
	// (issue #64).
	if data.Caching == nil {
		data.Caching = &cachingModel{}
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

	// Write-only attributes are not available in the plan; read them from the config.
	var config configModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyWriteOnlyAttributes(&data, &config)

	validateCachingCombination(&data, &config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	params := hyperdrive.ConfigUpdateParams{
		Hyperdrive: r.buildSDKParams(&data),
	}
	shared.SetParamField(&params.AccountID, r.client.AccountID)

	// Config omission means caching is unmanaged. Fetch it immediately before
	// every full-replace PUT so a refresh-skipped update cannot overwrite a
	// newer remote value with stale state.
	unmanagedCaching := config.Caching == nil
	plannedCaching := data.Caching
	if unmanagedCaching {
		getParams := hyperdrive.ConfigGetParams{}
		shared.SetParamField(&getParams.AccountID, r.client.AccountID)
		current, err := r.client.Hyperdrive.Configs.Get(ctx, data.ID.ValueString(), getParams)
		if err != nil {
			// Abort on any GET error, including 404. A genuinely deleted
			// resource would make the subsequent PUT fail anyway, so
			// aborting here loses nothing; proceeding without caching on a
			// transient 404 would silently reset remote caching to
			// Cloudflare defaults via the full-replace PUT.
			resp.Diagnostics.AddError("Failed to read Hyperdrive config before update", err.Error())
			return
		}
		shared.SetParamField(&params.Hyperdrive.Caching, cachingParamFromResponse(&current.Caching))
	}

	result, err := r.client.Hyperdrive.Configs.Update(ctx, data.ID.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Hyperdrive config", err.Error())
		return
	}

	if unmanagedCaching {
		// Keep the applied state equal to the known plan value. The live caching
		// value was preserved in the PUT above, but reflecting it here could
		// contradict a stale plan produced with refresh disabled. The next Read
		// will backfill the live value.
		data.Caching = nil
	}
	r.mapResponseToModel(result, &data)
	if unmanagedCaching {
		data.Caching = plannedCaching
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *configResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := hyperdrive.ConfigDeleteParams{}
	shared.SetParamField(&params.AccountID, r.client.AccountID)
	_, err := r.client.Hyperdrive.Configs.Delete(ctx, data.ID.ValueString(), params)
	if err != nil {
		if shared.IsNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete Hyperdrive config", err.Error())
		return
	}
}

func (r *configResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Read now unconditionally seeds an empty caching block before mapping
	// the API response (see Read), so the ImportState-time seeding from
	// issue #60/#61 is no longer needed: the Read that follows import
	// populates `caching` on its own regardless of the freshly-imported
	// state's initial content (issue #64).
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *configResource) mapResponseToModel(result *hyperdrive.Hyperdrive, data *configModel) {
	data.ID = types.StringValue(result.ID)
	data.Name = types.StringValue(result.Name)

	if data.Origin == nil {
		data.Origin = &originModel{}
	}
	data.Origin.Host = types.StringValue(result.Origin.Host)
	if result.Origin.Port != 0 {
		data.Origin.Port = types.Int64Value(result.Origin.Port)
	} else if data.Origin.Port.IsNull() || data.Origin.Port.IsUnknown() {
		// Access-protected origins (behind a Cloudflare Tunnel) have no `port`
		// field in the API request or response. Fall back to the schema
		// default so imported state matches the plan's computed default and
		// doesn't produce an inconsistent-result error or a permanent diff.
		data.Origin.Port = types.Int64Value(5432)
	}
	// If port is already known (non-null, non-unknown) in data.Origin, leave
	// it as-is; the access-protected origin's response omits port entirely.
	data.Origin.Database = types.StringValue(result.Origin.Database)
	data.Origin.User = types.StringValue(result.Origin.User)
	data.Origin.Scheme = types.StringValue(string(result.Origin.Scheme))
	// Reflect access_client_id as-is (including its absence) so that drift
	// from an access-protected origin being switched to a public origin is
	// detected. access_client_secret/access_client_secret_wo are write-only
	// and never returned by the API, so they are intentionally left untouched.
	if result.Origin.AccessClientID != "" {
		data.Origin.AccessClientID = types.StringValue(result.Origin.AccessClientID)
	} else {
		data.Origin.AccessClientID = types.StringNull()
	}

	if data.Caching != nil {
		data.Caching.Disabled = types.BoolValue(result.Caching.Disabled)
		if result.Caching.Disabled {
			// Disabling caching discards max_age / stale_while_revalidate
			// server-side and the API omits them from responses (the SDK
			// then reports zero values). Null keeps state consistent with
			// the plan (see nullWhenCachingDisabledModifier) and drift-free
			// across refreshes (issue #58).
			data.Caching.MaxAge = types.Int64Null()
			data.Caching.StaleWhileRevalidate = types.Int64Null()
		} else {
			data.Caching.MaxAge = types.Int64Value(result.Caching.MaxAge)
			data.Caching.StaleWhileRevalidate = types.Int64Value(result.Caching.StaleWhileRevalidate)
		}
	}

	mapMTLSResponseToModel(&result.MTLS, data)

	if result.OriginConnectionLimit > 0 {
		data.OriginConnectionLimit = types.Int64Value(result.OriginConnectionLimit)
	}
}

// mapMTLSResponseToModel reflects the API's mtls response onto data.MTLS.
func mapMTLSResponseToModel(mtls *hyperdrive.HyperdriveMTLS, data *configModel) {
	if mtls.CACertificateID != "" || mtls.MTLSCertificateID != "" || mtls.Sslmode != "" {
		// The API reports mtls configuration: reflect it faithfully so drift
		// (including fields that were cleared remotely) is detected.
		if data.MTLS == nil {
			data.MTLS = &mtlsModel{}
		}
		if mtls.CACertificateID != "" {
			data.MTLS.CACertificateID = types.StringValue(mtls.CACertificateID)
		} else {
			data.MTLS.CACertificateID = types.StringNull()
		}
		if mtls.MTLSCertificateID != "" {
			data.MTLS.MTLSCertificateID = types.StringValue(mtls.MTLSCertificateID)
		} else {
			data.MTLS.MTLSCertificateID = types.StringNull()
		}
		if mtls.Sslmode != "" {
			data.MTLS.SSLMode = types.StringValue(mtls.Sslmode)
		} else {
			data.MTLS.SSLMode = types.StringNull()
		}
		return
	}

	if data.MTLS != nil && data.MTLS.CACertificateID.IsNull() && data.MTLS.MTLSCertificateID.IsNull() && data.MTLS.SSLMode.IsNull() {
		// The plan/state has an explicit but empty mtls block (`mtls = {}`).
		// Keep it as-is rather than nulling it out, otherwise Terraform would
		// see an "inconsistent result after apply" error when the plan
		// expects a non-null (but all-null-fields) mtls object.
		return
	}

	// The API reports no mtls configuration: clear it so that mtls being
	// removed remotely is detected as drift instead of leaving stale
	// values in state.
	data.MTLS = nil
}
