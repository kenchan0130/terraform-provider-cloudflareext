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
	PasswordWOVersion           types.Int64  `tfsdk:"password_wo_version"`
	Scheme                      types.String `tfsdk:"scheme"`
	AccessClientID              types.String `tfsdk:"access_client_id"`
	AccessClientSecret          types.String `tfsdk:"access_client_secret"`
	AccessClientSecretWO        types.String `tfsdk:"access_client_secret_wo"`
	AccessClientSecretWOVersion types.Int64  `tfsdk:"access_client_secret_wo_version"`
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

type apiCreateRequest struct {
	Name                  string      `json:"name"`
	Origin                apiOrigin   `json:"origin"`
	Caching               *apiCaching `json:"caching,omitempty"`
	MTLS                  *apiMTLS    `json:"mtls,omitempty"`
	OriginConnectionLimit *int64      `json:"origin_connection_limit,omitempty"`
}

type apiOrigin struct {
	Host               string `json:"host"`
	Port               int64  `json:"port"`
	Database           string `json:"database"`
	User               string `json:"user"`
	Password           string `json:"password"`
	Scheme             string `json:"scheme"`
	AccessClientID     string `json:"access_client_id,omitempty"`
	AccessClientSecret string `json:"access_client_secret,omitempty"`
}

type apiCaching struct {
	Disabled             *bool  `json:"disabled,omitempty"`
	MaxAge               *int64 `json:"max_age,omitempty"`
	StaleWhileRevalidate *int64 `json:"stale_while_revalidate,omitempty"`
}

type apiMTLS struct {
	CACertificateID   string `json:"ca_certificate_id,omitempty"`
	MTLSCertificateID string `json:"mtls_certificate_id,omitempty"`
	SSLMode           string `json:"sslmode,omitempty"`
}

// apiResponse is the Cloudflare API response.
// The password and access_client_secret fields are never returned in GET responses.
type apiResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Origin struct {
		Host           string `json:"host"`
		Port           int64  `json:"port"`
		Database       string `json:"database"`
		User           string `json:"user"`
		Scheme         string `json:"scheme"`
		AccessClientID string `json:"access_client_id,omitempty"`
	} `json:"origin"`
	Caching struct {
		Disabled             bool  `json:"disabled"`
		MaxAge               int64 `json:"max_age"`
		StaleWhileRevalidate int64 `json:"stale_while_revalidate"`
	} `json:"caching"`
	MTLS *struct {
		CACertificateID   string `json:"ca_certificate_id,omitempty"`
		MTLSCertificateID string `json:"mtls_certificate_id,omitempty"`
		SSLMode           string `json:"sslmode,omitempty"`
	} `json:"mtls,omitempty"`
	OriginConnectionLimit int64 `json:"origin_connection_limit"`
}
