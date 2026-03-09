package secretsstore

import "github.com/hashicorp/terraform-plugin-framework/types"

// storeModel is the Terraform resource/data source data model for a Secrets Store.
type storeModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

type apiStoreCreateRequest struct {
	Name string `json:"name"`
}

type apiStoreResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

// secretModel is the Terraform resource data model for a Secrets Store secret.
type secretModel struct {
	ID       types.String `tfsdk:"id"`
	StoreID  types.String `tfsdk:"store_id"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	ValueWO  types.String `tfsdk:"value_wo"`
	Comment  types.String `tfsdk:"comment"`
	Scopes   types.List   `tfsdk:"scopes"`
	Status   types.String `tfsdk:"status"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}

type apiSecretCreateRequest struct {
	Name    string   `json:"name"`
	Value   string   `json:"value"`
	Scopes  []string `json:"scopes"`
	Comment string   `json:"comment,omitempty"`
}

type apiSecretUpdateRequest struct {
	Name    string   `json:"name,omitempty"`
	Value   string   `json:"value,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

// apiSecretResponse is the Secrets Store API response.
// The value field is never returned in GET responses.
type apiSecretResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	StoreID  string `json:"store_id"`
	Comment  string `json:"comment"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}
