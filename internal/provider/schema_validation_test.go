package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProviderResourceSchemasValidateImplementation(t *testing.T) {
	ctx := context.Background()
	p := New("test")()
	for _, resourceFactory := range p.Resources(ctx) {
		providerResource := resourceFactory()
		var resp resource.SchemaResponse
		providerResource.Schema(ctx, resource.SchemaRequest{}, &resp)
		for _, diag := range resp.Schema.ValidateImplementation(ctx) {
			t.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
	}
}

func TestProviderDataSourceSchemasValidateImplementation(t *testing.T) {
	ctx := context.Background()
	p := New("test")()
	for _, dataSourceFactory := range p.DataSources(ctx) {
		providerDataSource := dataSourceFactory()
		var resp datasource.SchemaResponse
		providerDataSource.Schema(ctx, datasource.SchemaRequest{}, &resp)
		for _, diag := range resp.Schema.ValidateImplementation(ctx) {
			t.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
	}
}
