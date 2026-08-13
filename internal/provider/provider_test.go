package provider

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/meigma/terraform-provider-example/internal/core"
)

func TestProviderMetadata(t *testing.T) {
	t.Parallel()

	var resp provider.MetadataResponse

	New("0.1.0")().Metadata(t.Context(), provider.MetadataRequest{}, &resp)

	assert.Equal(t, "example", resp.TypeName, "the type name is the prefix every resource inherits")
	assert.Equal(t, "0.1.0", resp.Version)
}

// TestProviderConfigure covers where the store path comes from. It is not
// parallel because the environment fallback is process-wide state.
func TestProviderConfigure(t *testing.T) {
	tests := []struct {
		name      string
		attribute types.String
		env       string
		wantPaths []string
	}{
		{
			name:      "uses the path from the provider block",
			attribute: types.StringValue("configured/items.json"),
		},
		{
			name: "falls back to the environment when the block omits the path",
			env:  "environment/items.json",
		},
		{
			name:      "prefers the provider block over the environment",
			attribute: types.StringValue("configured/items.json"),
			env:       "environment/items.json",
		},
		{
			name:      "reports a missing path against the store_path attribute",
			wantPaths: []string{"store_path"},
		},
		{
			name:      "refuses to configure while the path is unknown",
			attribute: types.StringUnknown(),
			wantPaths: []string{"store_path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			wantFile := ""
			if tt.env != "" {
				wantFile = filepath.Join(dir, tt.env)
				t.Setenv(storePathEnvVar, wantFile)
			}

			config := providerModel{StorePath: tt.attribute}
			if tt.attribute.ValueString() != "" {
				wantFile = filepath.Join(dir, tt.attribute.ValueString())
				config.StorePath = types.StringValue(wantFile)
			}

			resp := configureProvider(t, config)

			if tt.wantPaths != nil {
				assert.Equal(t, tt.wantPaths, errorPaths(resp.Diagnostics))
				assert.Nil(t, resp.ResourceData, "a failed configure must hand out no store")

				return
			}

			require.False(t, resp.Diagnostics.HasError(), "configure reported: %v", resp.Diagnostics)
			assert.Equal(t, resp.ResourceData, resp.DataSourceData,
				"resources and data sources must share one store")

			store, ok := resp.ResourceData.(core.Store)
			require.True(t, ok, "the provider must hand out something satisfying the store port")

			_, err := store.Create(t.Context(), core.Item{Name: "web"})
			require.NoError(t, err)

			assert.FileExists(t, wantFile, "the store must write to the configured path")
		})
	}
}

// configureProvider runs Configure against a provider block holding config.
func configureProvider(t *testing.T, config providerModel) provider.ConfigureResponse {
	t.Helper()

	subject := New("test")()

	var schemaResp provider.SchemaResponse

	subject.Schema(t.Context(), provider.SchemaRequest{}, &schemaResp)

	providerSchema := schemaResp.Schema
	empty := tfsdk.State{Schema: providerSchema, Raw: emptyState(t.Context(), providerSchema)}

	var resp provider.ConfigureResponse

	subject.Configure(t.Context(), provider.ConfigureRequest{
		Config: tfsdk.Config{Schema: providerSchema, Raw: fill(t, empty, &config)},
	}, &resp)

	return resp
}

// TestProviderNamesItemAfterItsTypeName pins the names a configuration writes.
// Both the resource and the data source derive them from the provider's type
// name, so changing that renames everything a user has already written.
func TestProviderNamesItemAfterItsTypeName(t *testing.T) {
	t.Parallel()

	subject := New("test")()

	resources := subject.Resources(t.Context())
	dataSources := subject.DataSources(t.Context())
	require.Len(t, resources, 1)
	require.Len(t, dataSources, 1)

	var resourceResp resource.MetadataResponse

	resources[0]().Metadata(
		t.Context(),
		resource.MetadataRequest{ProviderTypeName: TypeName},
		&resourceResp,
	)

	var dataSourceResp datasource.MetadataResponse

	dataSources[0]().Metadata(
		t.Context(),
		datasource.MetadataRequest{ProviderTypeName: TypeName},
		&dataSourceResp,
	)

	assert.Equal(t, "example_item", resourceResp.TypeName)
	assert.Equal(t, "example_item", dataSourceResp.TypeName)
}
