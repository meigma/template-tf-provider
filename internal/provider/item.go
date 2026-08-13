package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/meigma/terraform-provider-example/internal/core"
)

// idAttribute is the schema attribute holding the store-assigned identifier.
// Unlike the others it has no matching [core.Field]: nothing about it comes
// from configuration, so nothing about it can fail validation.
const idAttribute = "id"

// itemModel is the Terraform-side shape of an item. The resource and the data
// source share it because they describe the same object; only which attributes
// are computed differs between their schemas.
type itemModel struct {
	// ID is the store-assigned identifier, computed on create.
	ID types.String `tfsdk:"id"`

	// Name is the item's unique name.
	Name types.String `tfsdk:"name"`

	// Description is the item's optional summary.
	Description types.String `tfsdk:"description"`

	// Tags is the item's optional set of labels. A set is used rather than a
	// list so Terraform ignores ordering, which lets the store keep the tags
	// sorted on disk without provoking a diff.
	Tags types.Set `tfsdk:"tags"`
}

// newItemModel converts a stored item into its Terraform shape.
//
// An empty description becomes null rather than the empty string. The domain
// cannot tell the two apart, and returning "" for a configuration that omitted
// the attribute would show up as a permanent diff.
func newItemModel(item core.Item) (itemModel, diag.Diagnostics) {
	tags, diags := tagsValue(item.Tags)

	description := types.StringNull()
	if item.Description != "" {
		description = types.StringValue(string(item.Description))
	}

	return itemModel{
		ID:          types.StringValue(string(item.ID)),
		Name:        types.StringValue(string(item.Name)),
		Description: description,
		Tags:        tags,
	}, diags
}

// item converts the model into a validated domain item, reporting any rule it
// breaks as a diagnostic scoped to the attribute at fault. The returned item
// carries whatever ID the model held, which is empty before a create.
func (m itemModel) item(ctx context.Context) (core.Item, diag.Diagnostics) {
	tags, diags := modelTags(ctx, m.Tags)
	if diags.HasError() {
		return core.Item{}, diags
	}

	item, err := core.NewItem(
		core.Name(m.Name.ValueString()),
		core.Description(m.Description.ValueString()),
		tags,
	)
	if err != nil {
		diags.Append(validationDiagnostic(err))

		return core.Item{}, diags
	}

	item.ID = core.ID(m.ID.ValueString())

	return item, diags
}

// modelTags reads a tag set into the domain's slice form, keeping a null set
// distinct from an empty one.
func modelTags(ctx context.Context, set types.Set) ([]core.Tag, diag.Diagnostics) {
	if set.IsNull() || set.IsUnknown() {
		return nil, nil
	}

	var values []string

	diags := set.ElementsAs(ctx, &values, false)
	if diags.HasError() {
		return nil, diags
	}

	tags := make([]core.Tag, len(values))
	for index, value := range values {
		tags[index] = core.Tag(value)
	}

	return tags, diags
}

// tagsValue builds a Terraform set from the domain's slice form, mapping a nil
// slice to a null set and an empty slice to an empty set.
func tagsValue(tags []core.Tag) (types.Set, diag.Diagnostics) {
	if tags == nil {
		return types.SetNull(types.StringType), nil
	}

	values := make([]attr.Value, len(tags))
	for index, tag := range tags {
		values[index] = types.StringValue(string(tag))
	}

	return types.SetValue(types.StringType, values)
}

// validationDiagnostic turns a domain validation failure into a diagnostic
// pointing at the attribute that caused it.
//
// The mapping is a plain lookup because [core.Field] values are spelled the
// same as the schema attributes they describe. Renaming an attribute means
// renaming the matching field constant.
func validationDiagnostic(err error) diag.Diagnostic {
	var invalid *core.ValidationError
	if errors.As(err, &invalid) {
		return diag.NewAttributeErrorDiagnostic(
			path.Root(string(invalid.Field)),
			"Invalid item",
			invalid.Reason,
		)
	}

	return diag.NewErrorDiagnostic("Invalid item", err.Error())
}

// storeFromProviderData recovers the shared store the provider built during
// configuration.
//
// It returns false when the provider has not run yet, which the framework does
// on purpose during validation, and adds a diagnostic when the data is present
// but of an unexpected type.
func storeFromProviderData(providerData any, diags *diag.Diagnostics) (core.Store, bool) {
	if providerData == nil {
		return nil, false
	}

	store, ok := providerData.(core.Store)
	if !ok {
		diags.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected an item store, got %T. Please report this to the provider developers.", providerData),
		)

		return nil, false
	}

	return store, true
}
