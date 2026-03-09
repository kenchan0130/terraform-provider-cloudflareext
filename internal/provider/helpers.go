package provider

import "github.com/hashicorp/terraform-plugin-framework/path"

// schemaPath is an alias for path.Root.
func schemaPath(name string) path.Path {
	return path.Root(name)
}
