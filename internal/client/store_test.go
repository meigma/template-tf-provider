package client_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/meigma/terraform-provider-example/internal/client"
	"github.com/meigma/terraform-provider-example/internal/core"
)

// storeContext is one store over a scratch file that no other test can see.
type storeContext struct {
	// path is the store file itself. Its parent directory does not exist until
	// the store writes, which is deliberate: creating it is the store's job.
	path client.Path

	// store is the subject under test.
	store *client.Store
}

func newStoreContext(t *testing.T) *storeContext {
	t.Helper()

	path := client.Path(filepath.Join(t.TempDir(), "state", "items.json"))

	return &storeContext{path: path, store: client.New(path)}
}

// reopen returns a second store over the same file, standing in for the fresh
// provider process Terraform starts for every command.
func (c *storeContext) reopen() *client.Store {
	return client.New(c.path)
}

func TestStoreSurvivesReopening(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tc := newStoreContext(t)

	created, err := tc.store.Create(ctx, core.Item{
		Name:        "web-frontend",
		Description: "public entry point",
		Tags:        []core.Tag{"edge", "prod"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID, "the store must assign an identifier")

	reopened := tc.reopen()

	byID, err := reopened.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, byID, "the item read back must match the item stored")

	byName, err := reopened.GetByName(ctx, "web-frontend")
	require.NoError(t, err)
	assert.Equal(t, created, byName, "both lookups must return the same item")
}

func TestStoreMissingFileReadsAsEmpty(t *testing.T) {
	t.Parallel()

	tc := newStoreContext(t)

	_, err := tc.store.Get(t.Context(), "itm-nothing")

	require.ErrorIs(t, err, core.ErrNotFound, "an absent file is an empty store, not a failure")
	assert.NoFileExists(t, string(tc.path), "reading must not create the file")
}

func TestStoreTagShapesSurviveTheFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags []core.Tag
	}{
		{name: "absent tags stay absent", tags: nil},
		{name: "empty tags stay empty", tags: []core.Tag{}},
		{name: "populated tags keep their order", tags: []core.Tag{"edge", "prod"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			tc := newStoreContext(t)

			created, err := tc.store.Create(ctx, core.Item{Name: "web", Tags: tt.tags})
			require.NoError(t, err)

			read, err := tc.reopen().Get(ctx, created.ID)
			require.NoError(t, err)

			assert.Equal(t, tt.tags, read.Tags)
		})
	}
}

func TestStoreCreateRejectsADuplicateName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tc := newStoreContext(t)

	_, err := tc.store.Create(ctx, core.Item{Name: "web"})
	require.NoError(t, err)

	_, err = tc.store.Create(ctx, core.Item{Name: "web", Description: "a second one"})

	require.ErrorIs(t, err, core.ErrExists)
}

func TestStoreUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(existing core.Item) core.Item
		wantErr error
	}{
		{
			name: "rewrites the item in place",
			mutate: func(existing core.Item) core.Item {
				existing.Description = "rewritten"
				existing.Tags = []core.Tag{"staging"}

				return existing
			},
		},
		{
			name: "renames the item without changing its identifier",
			mutate: func(existing core.Item) core.Item {
				existing.Name = "web-renamed"

				return existing
			},
		},
		{
			name: "refuses a name another item already holds",
			mutate: func(existing core.Item) core.Item {
				existing.Name = "database"

				return existing
			},
			wantErr: core.ErrExists,
		},
		{
			name: "refuses an identifier the store does not hold",
			mutate: func(existing core.Item) core.Item {
				existing.ID = "itm-unknown"

				return existing
			},
			wantErr: core.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			tc := newStoreContext(t)

			existing, err := tc.store.Create(ctx, core.Item{Name: "web", Description: "original"})
			require.NoError(t, err)

			_, err = tc.store.Create(ctx, core.Item{Name: "database"})
			require.NoError(t, err)

			want := tt.mutate(existing)

			updated, err := tc.store.Update(ctx, want)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				stored, getErr := tc.reopen().Get(ctx, existing.ID)
				require.NoError(t, getErr)
				assert.Equal(t, existing, stored, "a rejected update must leave the item alone")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, want, updated)

			stored, err := tc.reopen().Get(ctx, existing.ID)
			require.NoError(t, err)
			assert.Equal(t, want, stored, "the update must reach the file")
		})
	}
}

func TestStoreDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tc := newStoreContext(t)

	created, err := tc.store.Create(ctx, core.Item{Name: "web"})
	require.NoError(t, err)

	require.NoError(t, tc.store.Delete(ctx, created.ID))

	_, err = tc.reopen().Get(ctx, created.ID)
	require.ErrorIs(t, err, core.ErrNotFound, "the item must be gone from the file")

	err = tc.store.Delete(ctx, created.ID)
	require.ErrorIs(t, err, core.ErrNotFound, "deleting twice must report the second one as missing")
}

func TestStoreLeavesNoScratchFilesBehind(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tc := newStoreContext(t)

	created, err := tc.store.Create(ctx, core.Item{Name: "web"})
	require.NoError(t, err)
	require.NoError(t, tc.store.Delete(ctx, created.ID))

	entries, err := os.ReadDir(filepath.Dir(string(tc.path)))
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	assert.Equal(t, []string{"items.json"}, names,
		"the atomic write must not leave temporary files in the store directory")
}

func TestStoreReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()

	tc := newStoreContext(t)

	require.NoError(t, os.MkdirAll(filepath.Dir(string(tc.path)), 0o700))
	require.NoError(t, os.WriteFile(string(tc.path), []byte("not json"), 0o600))

	_, err := tc.store.Get(t.Context(), "itm-anything")

	require.Error(t, err)
	require.NotErrorIs(t, err, core.ErrNotFound, "a corrupt file is a failure, not an empty store")
	assert.Contains(t, err.Error(), string(tc.path),
		"the failure must name the file that could not be read")
}
