package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/meigma/terraform-provider-example/internal/core"
	"github.com/meigma/terraform-provider-example/internal/core/mocks"
)

// dataSourceContext is a configured example_item data source wired to a mock
// store.
type dataSourceContext struct {
	// store is the port the data source was configured with.
	store *mocks.Store

	// dataSource is the subject under test.
	dataSource *itemDataSource

	// schema is the data source's schema, needed to build configs and states.
	schema schema.Schema
}

func newDataSourceContext(t *testing.T) *dataSourceContext {
	t.Helper()

	store := mocks.NewStore(t)
	subject := &itemDataSource{}

	var configureResp datasource.ConfigureResponse

	subject.Configure(
		t.Context(),
		datasource.ConfigureRequest{ProviderData: store},
		&configureResp,
	)
	require.False(t, configureResp.Diagnostics.HasError(),
		"configuring the data source with a store must succeed: %v", configureResp.Diagnostics)

	var schemaResp datasource.SchemaResponse

	subject.Schema(t.Context(), datasource.SchemaRequest{}, &schemaResp)

	return &dataSourceContext{store: store, dataSource: subject, schema: schemaResp.Schema}
}

func TestItemDataSourceRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupStore  func(store *mocks.Store)
		wantPaths   []string
		assertState func(t *testing.T, model itemModel)
	}{
		{
			name: "returns the item the name belongs to",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					GetByName(mock.Anything, core.Name("web-frontend")).
					Return(core.Item{
						ID:          "itm-abc",
						Name:        "web-frontend",
						Description: "public entry point",
						Tags:        []core.Tag{"edge", "prod"},
					}, nil)
			},
			assertState: func(t *testing.T, model itemModel) {
				t.Helper()

				assert.Equal(t, "itm-abc", model.ID.ValueString())
				assert.Equal(t, "public entry point", model.Description.ValueString())
				assert.Len(t, model.Tags.Elements(), 2, "the data source must expose the item's tags")
			},
		},
		{
			name: "blames the name attribute when no item has that name",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					GetByName(mock.Anything, core.Name("web-frontend")).
					Return(core.Item{}, fmt.Errorf("reading: %w", core.ErrNotFound))
			},
			wantPaths: []string{"name"},
		},
		{
			name: "reports an unexpected store failure against the data source",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					GetByName(mock.Anything, core.Name("web-frontend")).
					Return(core.Item{}, errStoreUnavailable)
			},
			wantPaths: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := newDataSourceContext(t)
			tt.setupStore(tc.store)

			empty := tfsdk.State{Schema: tc.schema, Raw: emptyState(t.Context(), tc.schema)}
			model := itemConfig{name: "web-frontend"}.model()
			config := tfsdk.Config{Schema: tc.schema, Raw: fill(t, empty, &model)}

			resp := datasource.ReadResponse{State: empty}
			tc.dataSource.Read(t.Context(), datasource.ReadRequest{Config: config}, &resp)

			if tt.wantPaths != nil {
				assert.Equal(t, tt.wantPaths, errorPaths(resp.Diagnostics))

				return
			}

			require.False(t, resp.Diagnostics.HasError(), "read reported: %v", resp.Diagnostics)
			tt.assertState(t, modelOf(t, resp.State))
		})
	}
}

func TestItemDataSourceWithoutAStoreDoesNothing(t *testing.T) {
	t.Parallel()

	subject := &itemDataSource{}

	var resp datasource.ConfigureResponse

	subject.Configure(t.Context(), datasource.ConfigureRequest{ProviderData: nil}, &resp)

	assert.False(t, resp.Diagnostics.HasError(),
		"the framework configures data sources before the provider runs; that is not an error")
	assert.Nil(t, subject.store)
}

func TestConfigureRejectsUnexpectedProviderData(t *testing.T) {
	t.Parallel()

	var resp datasource.ConfigureResponse

	store, ok := storeFromProviderData("not a store", &resp.Diagnostics)

	assert.False(t, ok)
	assert.Nil(t, store)
	assert.True(t, resp.Diagnostics.HasError(), "a store of the wrong type must be reported, not ignored")
}
