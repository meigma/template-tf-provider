package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TypeName is the provider's Terraform type name. It is the prefix Terraform
// derives resource and data source names from, so `example_thing` belongs to
// this provider.
const TypeName = "example"

// exampleProvider is the starter provider. It declares no configuration,
// resources, or data sources; a generated repository fills those in.
type exampleProvider struct {
	// version is the release version stamped into the binary at build time.
	// Terraform reports it in error messages and `terraform version` output.
	version string
}

// New returns a constructor for the example provider bound to version. The
// indirection matches the plugin server's API: it may build a fresh provider
// per Terraform run.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &exampleProvider{version: version}
	}
}

// Metadata reports the provider's type name and version to Terraform.
func (p *exampleProvider) Metadata(
	_ context.Context,
	_ provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = TypeName
	resp.Version = p.version
}

// Schema describes the provider block's configuration. The starter provider
// takes no configuration, so the schema is empty.
func (p *exampleProvider) Schema(
	_ context.Context,
	_ provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{}
}

// Configure prepares any shared client the resources and data sources need.
// The starter provider has nothing to configure.
func (p *exampleProvider) Configure(
	_ context.Context,
	_ provider.ConfigureRequest,
	_ *provider.ConfigureResponse,
) {
}

// Resources lists the resource constructors the provider offers.
func (p *exampleProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

// DataSources lists the data source constructors the provider offers.
func (p *exampleProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
