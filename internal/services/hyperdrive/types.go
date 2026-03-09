package hyperdrive

import "github.com/hashicorp/terraform-plugin-framework/types"

// configModel is the Terraform resource data model.
type configModel struct {
	ID     types.String  `tfsdk:"id"`
	Name   types.String  `tfsdk:"name"`
	Origin *originModel  `tfsdk:"origin"`
}

// originModel represents the origin (database connection) configuration.
type originModel struct {
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	Database   types.String `tfsdk:"database"`
	User       types.String `tfsdk:"user"`
	Password   types.String `tfsdk:"password"`
	PasswordWO types.String `tfsdk:"password_wo"`
	Scheme     types.String `tfsdk:"scheme"`
}

type apiCreateRequest struct {
	Name   string    `json:"name"`
	Origin apiOrigin `json:"origin"`
}

type apiOrigin struct {
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	Scheme   string `json:"scheme"`
}

// apiResponse is the Cloudflare API response.
// The password field is never returned in GET responses.
type apiResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Origin struct {
		Host     string `json:"host"`
		Port     int64  `json:"port"`
		Database string `json:"database"`
		User     string `json:"user"`
		Scheme   string `json:"scheme"`
	} `json:"origin"`
}
