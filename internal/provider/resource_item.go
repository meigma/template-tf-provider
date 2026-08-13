package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/meigma/terraform-provider-example/internal/core"
)

// itemResource is the example_item resource: full CRUD over one stored item.
type itemResource struct {
	// store is the port the resource reads and writes items through.
	store core.Store
}

// Compile-time proof that the resource implements every framework interface it
// advertises, including the optional one that receives the provider's store.
var (
	_ resource.Resource                = (*itemResource)(nil)
	_ resource.ResourceWithConfigure   = (*itemResource)(nil)
	_ resource.ResourceWithImportState = (*itemResource)(nil)
)

// newItemResource builds an unconfigured example_item resource. The framework
// calls it once per Terraform operation and then hands the instance its store
// through Configure.
func newItemResource() resource.Resource {
	return &itemResource{}
}

// Metadata reports the resource's Terraform type name, `example_item`.
func (r *itemResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_item"
}

// Schema describes the resource's attributes.
func (r *itemResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An item in the store. Renaming an item updates it in place; " +
			"the identifier stays the same for the life of the resource.",
		Attributes: map[string]schema.Attribute{
			idAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier assigned by the store when the item is created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			string(core.FieldName): schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Unique name for the item. Lowercase letters, digits, " +
					"hyphens, and underscores; must start with a letter.",
			},
			string(core.FieldDescription): schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-form summary of the item.",
			},
			string(core.FieldTags): schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Labels attached to the item. Lowercase letters, digits, and hyphens.",
			},
		},
	}
}

// Configure receives the store the provider built during configuration.
func (r *itemResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if store, ok := storeFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		r.store = store
	}
}

// Create stores the planned item and records the identifier the store assigned.
//
// The saved state is the plan with its identifier filled in rather than a model
// rebuilt from the store. Terraform rejects an applied value that differs from
// the planned one, and the domain guarantees the two agree: it rejects input it
// would have to rewrite instead of rewriting it.
func (r *itemResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan itemModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	item, diags := plan.item(ctx)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.store.Create(ctx, item)
	if err != nil {
		resp.Diagnostics.Append(writeDiagnostic("create", item.Name, err))

		return
	}

	plan.ID = types.StringValue(string(created.ID))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes state from the store, dropping the resource when the item is
// gone so Terraform plans to recreate it.
func (r *itemResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state itemModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	item, err := r.store.Get(ctx, core.ID(state.ID.ValueString()))

	switch {
	case errors.Is(err, core.ErrNotFound):
		resp.State.RemoveResource(ctx)

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

// Update writes the planned item over the stored one, keeping the identifier
// the resource was created with.
func (r *itemResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan, state itemModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID

	item, diags := plan.item(ctx)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.store.Update(ctx, item); err != nil {
		resp.Diagnostics.Append(writeDiagnostic("update", item.Name, err))

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the item. An item that is already gone is a success: the goal
// state — no such item — has been reached.
func (r *itemResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state itemModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.store.Delete(ctx, core.ID(state.ID.ValueString()))
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		resp.Diagnostics.AddError("Unable to delete item", err.Error())
	}
}

// ImportState adopts an item the store already holds into Terraform state.
//
// The value passed to `terraform import` is the store's identifier, which is
// all the resource needs: writing it to the id attribute is enough for the
// framework to call Read next and fill in the rest. An identifier the store
// does not hold leaves state empty, which the framework reports as an attempt
// to import something that is not there.
func (r *itemResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root(idAttribute), req, resp)
}

// writeDiagnostic reports a failed create or update, blaming the name
// attribute when the store rejected it as a duplicate.
func writeDiagnostic(action string, name core.Name, err error) diag.Diagnostic {
	if errors.Is(err, core.ErrExists) {
		return diag.NewAttributeErrorDiagnostic(
			path.Root(string(core.FieldName)),
			"Item name already in use",
			"Another item is already named "+string(name)+". Item names must be unique within a store.",
		)
	}

	return diag.NewErrorDiagnostic("Unable to "+action+" item", err.Error())
}
