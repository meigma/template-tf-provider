package provider

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/meigma/terraform-provider-example/internal/core"
	"github.com/meigma/terraform-provider-example/internal/core/mocks"
)

// errStoreUnavailable stands in for a failure the provider cannot interpret.
var errStoreUnavailable = errors.New("store unavailable")

// resourceContext is a configured example_item resource wired to a mock store.
type resourceContext struct {
	// store is the port the resource was configured with. It asserts its own
	// expectations during test cleanup.
	store *mocks.Store

	// resource is the subject under test.
	resource *itemResource

	// schema is the resource's schema, needed to build plans and states.
	schema schema.Schema
}

func newResourceContext(t *testing.T) *resourceContext {
	t.Helper()

	store := mocks.NewStore(t)
	subject := &itemResource{}

	var configureResp resource.ConfigureResponse

	subject.Configure(
		t.Context(),
		resource.ConfigureRequest{ProviderData: store},
		&configureResp,
	)
	require.False(t, configureResp.Diagnostics.HasError(),
		"configuring the resource with a store must succeed: %v", configureResp.Diagnostics)

	var schemaResp resource.SchemaResponse

	subject.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)

	return &resourceContext{store: store, resource: subject, schema: schemaResp.Schema}
}

// emptyState returns a null state carrying the resource's schema.
func (c *resourceContext) emptyState(t *testing.T) tfsdk.State {
	t.Helper()

	return tfsdk.State{Schema: c.schema, Raw: emptyState(t.Context(), c.schema)}
}

// stateOf returns a state holding the given configuration.
func (c *resourceContext) stateOf(t *testing.T, config itemConfig) tfsdk.State {
	t.Helper()

	model := config.model()

	return tfsdk.State{Schema: c.schema, Raw: fill(t, c.emptyState(t), &model)}
}

func TestItemResourceCreate(t *testing.T) {
	t.Parallel()

	stored := core.Item{
		ID:          "itm-abc",
		Name:        "web-frontend",
		Description: "public entry point",
		Tags:        []core.Tag{"edge", "prod"},
	}

	tests := []struct {
		name        string
		config      itemConfig
		setupStore  func(store *mocks.Store)
		wantPaths   []string
		assertState func(t *testing.T, model itemModel)
	}{
		{
			name: "stores the planned item and records the identifier the store assigned",
			config: itemConfig{
				id:          types.StringUnknown(),
				name:        "web-frontend",
				description: "public entry point",
				tags:        []string{"prod", "edge"},
			},
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Create(mock.Anything, core.Item{
						Name:        "web-frontend",
						Description: "public entry point",
						Tags:        []core.Tag{"edge", "prod"},
					}).
					Return(stored, nil)
			},
			assertState: func(t *testing.T, model itemModel) {
				t.Helper()

				assert.Equal(t, "itm-abc", model.ID.ValueString(), "the store's identifier must reach state")
				assert.Equal(t, "web-frontend", model.Name.ValueString())
				assert.Equal(t, "public entry point", model.Description.ValueString())
				assert.Len(t, model.Tags.Elements(), 2)
			},
		},
		{
			name: "leaves an omitted description and tags null in state",
			config: itemConfig{
				id:   types.StringUnknown(),
				name: "web-frontend",
			},
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Create(mock.Anything, core.Item{Name: "web-frontend"}).
					Return(core.Item{ID: "itm-abc", Name: "web-frontend"}, nil)
			},
			assertState: func(t *testing.T, model itemModel) {
				t.Helper()

				assert.True(t, model.Description.IsNull(), "an omitted description must stay null")
				assert.True(t, model.Tags.IsNull(), "omitted tags must stay null")
			},
		},
		{
			name:   "blames the name attribute when the store already holds that name",
			config: itemConfig{id: types.StringUnknown(), name: "web-frontend"},
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Create(mock.Anything, mock.Anything).
					Return(core.Item{}, fmt.Errorf("creating item: %w", core.ErrExists))
			},
			wantPaths: []string{"name"},
		},
		{
			name:      "rejects an invalid name without reaching the store",
			config:    itemConfig{id: types.StringUnknown(), name: "Web-Frontend"},
			wantPaths: []string{"name"},
		},
		{
			name:      "blames the tags attribute for an invalid tag",
			config:    itemConfig{id: types.StringUnknown(), name: "web-frontend", tags: []string{"Prod"}},
			wantPaths: []string{"tags"},
		},
		{
			name:   "reports an unexpected store failure against the resource",
			config: itemConfig{id: types.StringUnknown(), name: "web-frontend"},
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Create(mock.Anything, mock.Anything).
					Return(core.Item{}, errStoreUnavailable)
			},
			wantPaths: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := newResourceContext(t)
			if tt.setupStore != nil {
				tt.setupStore(tc.store)
			}

			resp := resource.CreateResponse{State: tc.emptyState(t)}
			tc.resource.Create(t.Context(), resource.CreateRequest{
				Plan: tfsdk.Plan{Schema: tc.schema, Raw: tc.stateOf(t, tt.config).Raw},
			}, &resp)

			if tt.wantPaths != nil {
				assert.Equal(t, tt.wantPaths, errorPaths(resp.Diagnostics))
				assert.True(t, resp.State.Raw.IsNull(), "a failed create must not write state")

				return
			}

			require.False(t, resp.Diagnostics.HasError(), "create reported: %v", resp.Diagnostics)
			tt.assertState(t, modelOf(t, resp.State))
		})
	}
}

func TestItemResourceRead(t *testing.T) {
	t.Parallel()

	priorState := itemConfig{id: types.StringValue("itm-abc"), name: "web-frontend"}

	tests := []struct {
		name        string
		setupStore  func(store *mocks.Store)
		assertState func(t *testing.T, state tfsdk.State)
	}{
		{
			name: "refreshes every attribute from the store",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Get(mock.Anything, core.ID("itm-abc")).
					Return(core.Item{
						ID:          "itm-abc",
						Name:        "web-renamed",
						Description: "changed outside Terraform",
						Tags:        []core.Tag{"edge"},
					}, nil)
			},
			assertState: func(t *testing.T, state tfsdk.State) {
				t.Helper()

				model := modelOf(t, state)
				assert.Equal(t, "web-renamed", model.Name.ValueString(), "drift must reach state")
				assert.Equal(t, "changed outside Terraform", model.Description.ValueString())
				assert.Len(t, model.Tags.Elements(), 1)
			},
		},
		{
			name: "reports an item with no description as null rather than empty",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Get(mock.Anything, core.ID("itm-abc")).
					Return(core.Item{ID: "itm-abc", Name: "web-frontend"}, nil)
			},
			assertState: func(t *testing.T, state tfsdk.State) {
				t.Helper()

				model := modelOf(t, state)
				assert.True(t, model.Description.IsNull(),
					"an empty description must read back as null so it does not show as a diff")
				assert.True(t, model.Tags.IsNull())
			},
		},
		{
			name: "drops the resource when the item is gone",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Get(mock.Anything, core.ID("itm-abc")).
					Return(core.Item{}, fmt.Errorf("reading: %w", core.ErrNotFound))
			},
			assertState: func(t *testing.T, state tfsdk.State) {
				t.Helper()

				assert.True(t, state.Raw.IsNull(), "a missing item must clear state so Terraform recreates it")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := newResourceContext(t)
			tt.setupStore(tc.store)

			state := tc.stateOf(t, priorState)
			resp := resource.ReadResponse{State: state}
			tc.resource.Read(t.Context(), resource.ReadRequest{State: state}, &resp)

			require.False(t, resp.Diagnostics.HasError(), "read reported: %v", resp.Diagnostics)
			tt.assertState(t, resp.State)
		})
	}
}

func TestItemResourceUpdateKeepsTheIdentifierFromState(t *testing.T) {
	t.Parallel()

	tc := newResourceContext(t)

	updated := core.Item{ID: "itm-abc", Name: "web-renamed", Description: "now with a summary"}
	tc.store.EXPECT().Update(mock.Anything, updated).Return(updated, nil)

	state := tc.stateOf(t, itemConfig{id: types.StringValue("itm-abc"), name: "web-frontend"})
	plan := tc.stateOf(t, itemConfig{
		id:          types.StringValue("itm-abc"),
		name:        "web-renamed",
		description: "now with a summary",
	})

	resp := resource.UpdateResponse{State: state}
	tc.resource.Update(t.Context(), resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: tc.schema, Raw: plan.Raw},
		State: state,
	}, &resp)

	require.False(t, resp.Diagnostics.HasError(), "update reported: %v", resp.Diagnostics)

	model := modelOf(t, resp.State)
	assert.Equal(t, "itm-abc", model.ID.ValueString(), "an update must not change the identifier")
	assert.Equal(t, "web-renamed", model.Name.ValueString())
}

// TestItemResourceImportState drives import the way the framework does:
// ImportState writes the identifier, and the framework then calls Read to
// populate everything else. Testing the two together is the only way to see
// that import reuses the read path rather than duplicating it.
func TestItemResourceImportState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupStore  func(store *mocks.Store)
		assertState func(t *testing.T, state tfsdk.State)
	}{
		{
			name: "adopts an existing item through the read path",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Get(mock.Anything, core.ID("itm-abc")).
					Return(core.Item{
						ID:          "itm-abc",
						Name:        "web-frontend",
						Description: "public entry point",
						Tags:        []core.Tag{"edge", "prod"},
					}, nil)
			},
			assertState: func(t *testing.T, state tfsdk.State) {
				t.Helper()

				model := modelOf(t, state)
				assert.Equal(t, "itm-abc", model.ID.ValueString())
				assert.Equal(t, "web-frontend", model.Name.ValueString(),
					"import must fill in the attributes the identifier alone does not carry")
				assert.Equal(t, "public entry point", model.Description.ValueString())
				assert.Len(t, model.Tags.Elements(), 2)
			},
		},
		{
			name: "leaves state empty for an identifier the store does not hold",
			setupStore: func(store *mocks.Store) {
				store.EXPECT().
					Get(mock.Anything, core.ID("itm-abc")).
					Return(core.Item{}, fmt.Errorf("reading: %w", core.ErrNotFound))
			},
			assertState: func(t *testing.T, state tfsdk.State) {
				t.Helper()

				assert.True(t, state.Raw.IsNull(),
					"empty state is what makes the framework report the object as not importable")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := newResourceContext(t)
			tt.setupStore(tc.store)

			importResp := resource.ImportStateResponse{State: tc.emptyState(t)}
			tc.resource.ImportState(
				t.Context(),
				resource.ImportStateRequest{ID: "itm-abc"},
				&importResp,
			)
			require.False(t, importResp.Diagnostics.HasError(),
				"import reported: %v", importResp.Diagnostics)
			require.Equal(t, "itm-abc", modelOf(t, importResp.State).ID.ValueString(),
				"import must write the identifier it was given")

			readResp := resource.ReadResponse{State: importResp.State}
			tc.resource.Read(
				t.Context(),
				resource.ReadRequest{State: importResp.State},
				&readResp,
			)
			require.False(t, readResp.Diagnostics.HasError(), "read reported: %v", readResp.Diagnostics)

			tt.assertState(t, readResp.State)
		})
	}
}

func TestItemResourceDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		deleteErr error
		wantError bool
	}{
		{name: "removes the item"},
		{name: "treats an already deleted item as success", deleteErr: core.ErrNotFound},
		{name: "reports any other failure", deleteErr: errStoreUnavailable, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := newResourceContext(t)
			tc.store.EXPECT().Delete(mock.Anything, core.ID("itm-abc")).Return(tt.deleteErr)

			state := tc.stateOf(t, itemConfig{id: types.StringValue("itm-abc"), name: "web-frontend"})
			resp := resource.DeleteResponse{State: state}
			tc.resource.Delete(t.Context(), resource.DeleteRequest{State: state}, &resp)

			assert.Equal(t, tt.wantError, resp.Diagnostics.HasError(),
				"unexpected diagnostics: %v", resp.Diagnostics)
		})
	}
}
