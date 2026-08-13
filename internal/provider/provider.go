package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/meigma/terraform-provider-example/internal/client"
)

const (
	// TypeName is the provider's Terraform type name. It is the prefix
	// Terraform derives resource and data source names from, so `example_item`
	// belongs to this provider.
	TypeName = "example"

	// storePathAttribute is the provider block attribute naming the item
	// store's JSON file.
	storePathAttribute = "store_path"

	// storePathEnvVar supplies the store path when the provider block omits
	// it, which keeps the location out of checked-in configuration.
	storePathEnvVar = "EXAMPLE_STORE_PATH"
)

// exampleProvider is the provider Terraform talks to. It owns nothing but the
// configuration needed to build the store its resources and data sources share.
type exampleProvider struct {
	// version is the release version stamped into the binary at build time.
	// Terraform reports it in error messages and `terraform version` output.
	version string
}

// providerModel mirrors the provider block's schema.
type providerModel struct {
	// StorePath is the configured location of the item store's JSON file.
	StorePath types.String `tfsdk:"store_path"`
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

// Schema describes the provider block's configuration.
func (p *exampleProvider) Schema(
	_ context.Context,
	_ provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages example items in a JSON file on disk. " +
			"A real provider would point at a service here instead.",
		Attributes: map[string]schema.Attribute{
			storePathAttribute: schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Path to the JSON file the items are kept in. " +
					"May also be set with the `" + storePathEnvVar + "` environment variable. " +
					"The file and its parent directory are created on first write.",
			},
		},
	}
}

// Configure builds the item store the resources and data sources share.
//
// The store is handed over as a core.Store, not as a concrete type, so the
// resource and data source can be exercised against a generated mock. The path
// comes from the provider block when it is set and from the environment
// otherwise; neither being set is a configuration error rather than a default,
// because guessing a location for a file the provider writes to would be worse
// than saying so.
func (p *exampleProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var config providerModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.StorePath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(storePathAttribute),
			"Unknown item store path",
			"The provider cannot be configured while "+storePathAttribute+" is unknown. "+
				"Set it to a static value, or set the "+storePathEnvVar+" environment variable.",
		)

		return
	}

	storePath := config.StorePath.ValueString()
	if storePath == "" {
		storePath = os.Getenv(storePathEnvVar)
	}

	if storePath == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root(storePathAttribute),
			"Missing item store path",
			"The provider needs to know which file to keep items in. "+
				"Set "+storePathAttribute+" in the provider block, "+
				"or set the "+storePathEnvVar+" environment variable.",
		)

		return
	}

	store := client.New(client.Path(storePath))
	resp.ResourceData = store
	resp.DataSourceData = store
}

// Resources lists the resource constructors the provider offers.
func (p *exampleProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newItemResource,
	}
}

// DataSources lists the data source constructors the provider offers.
func (p *exampleProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newItemDataSource,
	}
}
