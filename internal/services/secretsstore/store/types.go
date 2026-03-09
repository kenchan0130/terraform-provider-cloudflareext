package store

import "github.com/hashicorp/terraform-plugin-framework/types"

type model struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

type apiCreateRequest struct {
	Name string `json:"name"`
}

type apiResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}
