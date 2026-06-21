package destination

import "github.com/hashicorp/terraform-plugin-framework/types"

type apiDestinationRequest struct {
	Configuration      apiDestinationRequestConfiguration `json:"configuration"`
	Enabled            bool                               `json:"enabled"`
	Name               string                             `json:"name,omitempty"`
	SkipPreflightCheck *bool                              `json:"skipPreflightCheck,omitempty"`
}

type apiDestinationRequestConfiguration struct {
	Headers        map[string]string `json:"headers"`
	LogpushDataset string            `json:"logpushDataset,omitempty"`
	Type           string            `json:"type"`
	URL            string            `json:"url"`
}

type apiDestinationResponse struct {
	Configuration apiDestinationResponseConfiguration `json:"configuration"`
	Enabled       bool                                `json:"enabled"`
	Name          string                              `json:"name"`
	Scripts       []string                            `json:"scripts"`
	Slug          string                              `json:"slug"`
}

type apiDestinationResponseConfiguration struct {
	DestinationConf string            `json:"destination_conf"`
	Headers         map[string]string `json:"headers"`
	LogpushDataset  string            `json:"logpushDataset"`
	LogpushJob      *float64          `json:"logpushJob"`
	Type            string            `json:"type"`
	URL             string            `json:"url"`
}

type model struct {
	ID                 types.String  `tfsdk:"id"`
	Slug               types.String  `tfsdk:"slug"`
	Name               types.String  `tfsdk:"name"`
	Enabled            types.Bool    `tfsdk:"enabled"`
	Type               types.String  `tfsdk:"type"`
	URL                types.String  `tfsdk:"url"`
	LogpushDataset     types.String  `tfsdk:"logpush_dataset"`
	Headers            types.Map     `tfsdk:"headers"`
	HeadersWO          types.Map     `tfsdk:"headers_wo"`
	HeadersWOVersion   types.String  `tfsdk:"headers_wo_version"`
	SkipPreflightCheck types.Bool    `tfsdk:"skip_preflight_check"`
	Scripts            types.List    `tfsdk:"scripts"`
	DestinationConf    types.String  `tfsdk:"destination_conf"`
	LogpushJob         types.Float64 `tfsdk:"logpush_job"`
}
