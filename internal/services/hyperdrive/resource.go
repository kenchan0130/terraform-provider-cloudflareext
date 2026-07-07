package hyperdrive

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v7/hyperdrive"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
		Description: "The caching configuration for the Hyperdrive.",
		Optional:    true,
		Attributes: map[string]schema.Attribute{
			"disabled": schema.BoolAttribute{
				Description: "Whether to disable caching of SQL responses. Defaults to `false`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"max_age": schema.Int64Attribute{
				Description: "The maximum duration (in seconds) for which items should persist in the cache. " +
					"Defaults to `60`. Maximum value is `3600`.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(60),
			},
			"stale_while_revalidate": schema.Int64Attribute{
				Description: "The number of seconds the cache may serve a stale response while revalidating. " +
					"Defaults to `15`.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(15),
			},
		},
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
		Validators: []validator.Int64{
			int64validator.Between(5, 100),
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
		cachingParam := hyperdrive.HyperdriveCachingHyperdriveHyperdriveCachingEnabledParam{}
		if !data.Caching.Disabled.IsNull() && !data.Caching.Disabled.IsUnknown() {
			shared.SetParamField(&cachingParam.Disabled, data.Caching.Disabled.ValueBool())
		}
		if !data.Caching.MaxAge.IsNull() && !data.Caching.MaxAge.IsUnknown() {
			shared.SetParamField(&cachingParam.MaxAge, data.Caching.MaxAge.ValueInt64())
		}
		if !data.Caching.StaleWhileRevalidate.IsNull() && !data.Caching.StaleWhileRevalidate.IsUnknown() {
			shared.SetParamField(&cachingParam.StaleWhileRevalidate, data.Caching.StaleWhileRevalidate.ValueInt64())
		}
		shared.SetParamField(&params.Caching, cachingParam)
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

	params := hyperdrive.ConfigUpdateParams{
		Hyperdrive: r.buildSDKParams(&data),
	}
	shared.SetParamField(&params.AccountID, r.client.AccountID)

	result, err := r.client.Hyperdrive.Configs.Update(ctx, data.ID.ValueString(), params)
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
		data.Caching.MaxAge = types.Int64Value(result.Caching.MaxAge)
		data.Caching.StaleWhileRevalidate = types.Int64Value(result.Caching.StaleWhileRevalidate)
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
