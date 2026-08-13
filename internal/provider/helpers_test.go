package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

// itemConfig is the configuration a test drives the resource or data source
// with. A zero field becomes a null attribute, which is what Terraform sends
// when a configuration leaves it out.
type itemConfig struct {
	// id is the identifier already in state, or unknown on a create.
	id types.String

	// name is the item's name.
	name string

	// description is the item's summary; empty means the attribute is null.
	description string

	// tags are the item's labels; nil means the attribute is null.
	tags []string
}

// model renders the configuration as the model the framework would decode.
func (c itemConfig) model() itemModel {
	model := itemModel{
		ID:          c.id,
		Name:        types.StringValue(c.name),
		Description: types.StringNull(),
		Tags:        types.SetNull(types.StringType),
	}

	if c.description != "" {
		model.Description = types.StringValue(c.description)
	}

	if c.tags != nil {
		values := make([]attr.Value, len(c.tags))
		for index, tag := range c.tags {
			values[index] = types.StringValue(tag)
		}

		model.Tags = types.SetValueMust(types.StringType, values)
	}

	return model
}

// emptyState returns a wholly null state for a schema, standing in for the
// prior state the framework hands a resource before anything exists.
//
// The schema is taken as its [attr.Type] because the framework's schema
// interface lives in an internal package and cannot be named from here.
func emptyState(ctx context.Context, schema interface{ Type() attr.Type }) tftypes.Value {
	return tftypes.NewValue(schema.Type().TerraformType(ctx), nil)
}

// fill writes a model into a state and returns the raw value it produced, so a
// test can hand the same shape back as a plan, a config, or a prior state.
func fill(t *testing.T, state tfsdk.State, model any) tftypes.Value {
	t.Helper()

	diags := state.Set(t.Context(), model)
	require.False(t, diags.HasError(), "building a test value: %v", diags)

	return state.Raw
}

// modelOf decodes a state back into an item model.
func modelOf(t *testing.T, state tfsdk.State) itemModel {
	t.Helper()

	var model itemModel

	diags := state.Get(t.Context(), &model)
	require.False(t, diags.HasError(), "reading state back: %v", diags)

	return model
}

// errorPaths returns the attribute each error diagnostic points at, using the
// empty string for a diagnostic that names no attribute. Comparing these is how
// the tests check that a failure is reported against the right line of
// configuration rather than against the resource as a whole.
func errorPaths(diags diag.Diagnostics) []string {
	paths := make([]string, 0, diags.ErrorsCount())

	for _, diagnostic := range diags.Errors() {
		withPath, ok := diagnostic.(diag.DiagnosticWithPath)
		if !ok {
			paths = append(paths, "")

			continue
		}

		paths = append(paths, withPath.Path().String())
	}

	return paths
}
