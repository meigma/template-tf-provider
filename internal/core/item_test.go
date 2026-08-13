package core_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/meigma/terraform-provider-example/internal/core"
)

func TestNewItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		itemName     core.Name
		description  core.Description
		tags         []core.Tag
		wantTags     []core.Tag
		invalidField core.Field
	}{
		{
			name:     "accepts a plain lowercase name",
			itemName: "web",
		},
		{
			name:     "accepts hyphens and underscores between words",
			itemName: "web-frontend_v2",
		},
		{
			name:     "accepts a name of the maximum length",
			itemName: core.Name(strings.Repeat("a", core.MaxNameLength)),
		},
		{
			name:         "rejects an empty name",
			itemName:     "",
			invalidField: core.FieldName,
		},
		{
			name:         "rejects a name longer than the maximum",
			itemName:     core.Name(strings.Repeat("a", core.MaxNameLength+1)),
			invalidField: core.FieldName,
		},
		{
			name:         "rejects an uppercase name rather than lowercasing it",
			itemName:     "WebFrontend",
			invalidField: core.FieldName,
		},
		{
			name:         "rejects a name starting with a digit",
			itemName:     "1st-item",
			invalidField: core.FieldName,
		},
		{
			name:         "rejects a name ending in a separator",
			itemName:     "web-",
			invalidField: core.FieldName,
		},
		{
			name:         "rejects repeated separators",
			itemName:     "web--frontend",
			invalidField: core.FieldName,
		},
		{
			name:         "rejects surrounding whitespace rather than trimming it",
			itemName:     " web ",
			invalidField: core.FieldName,
		},
		{
			name:        "accepts a description of the maximum length",
			itemName:    "web",
			description: core.Description(strings.Repeat("x", core.MaxDescriptionLength)),
		},
		{
			name:         "rejects a description longer than the maximum",
			itemName:     "web",
			description:  core.Description(strings.Repeat("x", core.MaxDescriptionLength+1)),
			invalidField: core.FieldDescription,
		},
		{
			name:         "rejects a description with trailing whitespace",
			itemName:     "web",
			description:  "public entry point ",
			invalidField: core.FieldDescription,
		},
		{
			name:     "sorts and deduplicates tags",
			itemName: "web",
			tags:     []core.Tag{"prod", "edge", "prod"},
			wantTags: []core.Tag{"edge", "prod"},
		},
		{
			name:     "leaves absent tags absent",
			itemName: "web",
			tags:     nil,
			wantTags: nil,
		},
		{
			name:     "keeps an empty tag set distinct from an absent one",
			itemName: "web",
			tags:     []core.Tag{},
			wantTags: []core.Tag{},
		},
		{
			name:         "rejects an uppercase tag",
			itemName:     "web",
			tags:         []core.Tag{"Prod"},
			invalidField: core.FieldTags,
		},
		{
			name:         "rejects an empty tag",
			itemName:     "web",
			tags:         []core.Tag{"prod", ""},
			invalidField: core.FieldTags,
		},
		{
			name:         "rejects an underscore in a tag",
			itemName:     "web",
			tags:         []core.Tag{"needs_review"},
			invalidField: core.FieldTags,
		},
		{
			name:         "rejects more tags than an item may carry",
			itemName:     "web",
			tags:         manyTags(core.MaxTags + 1),
			invalidField: core.FieldTags,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := core.NewItem(tt.itemName, tt.description, tt.tags)

			if tt.invalidField != "" {
				var invalid *core.ValidationError

				require.ErrorAs(t, err, &invalid, "expected a validation error naming a field")
				assert.Equal(t, tt.invalidField, invalid.Field, "wrong field blamed for the failure")
				assert.NotEmpty(t, invalid.Reason, "a validation error must explain itself")
				assert.Equal(t, core.Item{}, item, "a rejected item must not be returned")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.itemName, item.Name)
			assert.Equal(t, tt.description, item.Description)
			assert.Equal(t, tt.wantTags, item.Tags)
			assert.Empty(t, item.ID, "only a store assigns an identifier")
		})
	}
}

func TestItemNormalizeIsIdempotent(t *testing.T) {
	t.Parallel()

	item := core.Item{Name: "web", Tags: []core.Tag{"prod", "edge", "prod", "edge"}}

	once := item.Normalize()
	twice := once.Normalize()

	assert.Equal(t, []core.Tag{"edge", "prod"}, once.Tags)
	assert.Equal(t, once, twice, "normalizing an already normalized item must change nothing")
}

func TestItemNormalizeLeavesTheSourceAlone(t *testing.T) {
	t.Parallel()

	tags := []core.Tag{"prod", "edge"}
	item := core.Item{Name: "web", Tags: tags}

	item.Normalize()

	assert.Equal(t, []core.Tag{"prod", "edge"}, tags, "normalizing must not reorder the caller's slice")
}

// manyTags builds count distinct, individually valid tags.
func manyTags(count int) []core.Tag {
	tags := make([]core.Tag, count)
	for index := range tags {
		tags[index] = core.Tag("tag-" + string(rune('a'+index%26)) + strings.Repeat("z", index/26))
	}

	return tags
}
