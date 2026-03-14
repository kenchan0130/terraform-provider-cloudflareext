package secret

import "github.com/hashicorp/terraform-plugin-framework/types"

type model struct {
	ID             types.String `tfsdk:"id"`
	StoreID        types.String `tfsdk:"store_id"`
	Name           types.String `tfsdk:"name"`
	Value          types.String `tfsdk:"value"`
	ValueWO        types.String `tfsdk:"value_wo"`
	ValueWOVersion types.String `tfsdk:"value_wo_version"`
	Comment        types.String `tfsdk:"comment"`
	Scopes         types.List   `tfsdk:"scopes"`
	Status         types.String `tfsdk:"status"`
	Created        types.String `tfsdk:"created"`
	Modified       types.String `tfsdk:"modified"`
}
