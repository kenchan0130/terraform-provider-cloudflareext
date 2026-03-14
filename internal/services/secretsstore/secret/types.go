package secret

import "github.com/hashicorp/terraform-plugin-framework/types"

type model struct {
	ID             types.String `tfsdk:"id"`
	StoreID        types.String `tfsdk:"store_id"`
	Name           types.String `tfsdk:"name"`
	Value          types.String `tfsdk:"value"`
	ValueWO        types.String `tfsdk:"value_wo"`
	ValueWOVersion types.Int64  `tfsdk:"value_wo_version"`
	Comment        types.String `tfsdk:"comment"`
	Scopes         types.List   `tfsdk:"scopes"`
	Status         types.String `tfsdk:"status"`
	Created        types.String `tfsdk:"created"`
	Modified       types.String `tfsdk:"modified"`
}

type apiCreateRequest struct {
	Name    string   `json:"name"`
	Value   string   `json:"value"`
	Scopes  []string `json:"scopes"`
	Comment string   `json:"comment,omitempty"`
}

type apiUpdateRequest struct {
	Name    string   `json:"name,omitempty"`
	Value   string   `json:"value,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

// apiResponse is the Secrets Store API response.
// The value field is never returned in GET responses.
type apiResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	StoreID  string `json:"store_id"`
	Comment  string `json:"comment"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}
