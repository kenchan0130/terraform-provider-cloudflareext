package secret

import "github.com/hashicorp/terraform-plugin-framework/types"

type model struct {
	ScriptName    types.String `tfsdk:"script_name"`
	Name          types.String `tfsdk:"name"`
	Text          types.String `tfsdk:"text"`
	TextWO        types.String `tfsdk:"text_wo"`
	TextWOVersion types.String `tfsdk:"text_wo_version"`
	Type          types.String `tfsdk:"type"`
}
