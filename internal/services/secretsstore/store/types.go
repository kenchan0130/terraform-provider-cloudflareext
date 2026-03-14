package store

import "github.com/hashicorp/terraform-plugin-framework/types"

type model struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Created  types.String `tfsdk:"created"`
	Modified types.String `tfsdk:"modified"`
}
