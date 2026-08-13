package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/meigma/terraform-provider-example/internal/core"
)

// itemDataSource is the example_item data source: a read-only lookup by name.
type itemDataSource struct {
	// store is the port the data source reads items through.
	store core.Store
}

// Compile-time proof that the data source implements every framework interface
// it advertises.
var (
	_ datasource.DataSource              = (*itemDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*itemDataSource)(nil)
)

// newItemDataSource builds an unconfigured example_item data source. The
// framework hands the instance its store through Configure.
func newItemDataSource() datasource.DataSource {
	return &itemDataSource{}
}

// Metadata reports the data source's Terraform type name, `example_item`.
func (d *itemDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_item"
}

// Schema describes the data source's attributes. Only the name is supplied;
// everything else is read from the store.
func (d *itemDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single item by name. It is an error if no item has that name.",
		Attributes: map[string]schema.Attribute{
			idAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier the store assigned to the item.",
			},
			string(core.FieldName): schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the item to look up.",
			},
			string(core.FieldDescription): schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The item's summary, null when it has none.",
			},
			string(core.FieldTags): schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The item's labels, null when it has none.",
			},
		},
	}
}

// Configure receives the store the provider built during configuration.
func (d *itemDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	if store, ok := storeFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		d.store = store
	}
}

// Read looks the item up by name and fills in the rest of the attributes.
func (d *itemDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var config itemModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	name := core.Name(config.Name.ValueString())

	item, err := d.store.GetByName(ctx, name)

	switch {
	case errors.Is(err, core.ErrNotFound):
		resp.Diagnostics.AddAttributeError(
			path.Root(string(core.FieldName)),
			"Item not found",
			"No item in the store is named "+string(name)+".",
		)

		return
	case err != nil:
		resp.Diagnostics.AddError("Unable to read item", err.Error())

		return
	}

	model, diags := newItemModel(item)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
