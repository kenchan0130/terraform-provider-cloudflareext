package hyperdrive

import "github.com/hashicorp/terraform-plugin-framework/types"

// configModel is the Terraform resource data model.
type configModel struct {
	ID                    types.String  `tfsdk:"id"`
	Name                  types.String  `tfsdk:"name"`
	Origin                *originModel  `tfsdk:"origin"`
	Caching               *cachingModel `tfsdk:"caching"`
	MTLS                  *mtlsModel    `tfsdk:"mtls"`
	OriginConnectionLimit types.Int64   `tfsdk:"origin_connection_limit"`
}

// originModel represents the origin (database connection) configuration.
type originModel struct {
	Host                        types.String `tfsdk:"host"`
	Port                        types.Int64  `tfsdk:"port"`
	Database                    types.String `tfsdk:"database"`
	User                        types.String `tfsdk:"user"`
	Password                    types.String `tfsdk:"password"`
	PasswordWO                  types.String `tfsdk:"password_wo"`
	PasswordWOVersion           types.String `tfsdk:"password_wo_version"`
	Scheme                      types.String `tfsdk:"scheme"`
	AccessClientID              types.String `tfsdk:"access_client_id"`
	AccessClientSecret          types.String `tfsdk:"access_client_secret"`
	AccessClientSecretWO        types.String `tfsdk:"access_client_secret_wo"`
	AccessClientSecretWOVersion types.String `tfsdk:"access_client_secret_wo_version"`
}

// cachingModel represents the caching configuration.
type cachingModel struct {
	Disabled             types.Bool  `tfsdk:"disabled"`
	MaxAge               types.Int64 `tfsdk:"max_age"`
	StaleWhileRevalidate types.Int64 `tfsdk:"stale_while_revalidate"`
}

// mtlsModel represents the mTLS configuration.
type mtlsModel struct {
	CACertificateID   types.String `tfsdk:"ca_certificate_id"`
	MTLSCertificateID types.String `tfsdk:"mtls_certificate_id"`
	SSLMode           types.String `tfsdk:"sslmode"`
}
