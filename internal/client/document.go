package client

import (
	"slices"

	"github.com/meigma/terraform-provider-example/internal/core"
)

// document is the on-disk shape of the whole store. Wrapping the items in an
// object rather than writing a bare array leaves room to add file-level fields
// later without breaking readers.
type document struct {
	// Items holds every stored item, in insertion order.
	Items []storedItem `json:"items"`
}

// indexByID returns the position of the item carrying id, or -1.
func (d document) indexByID(id core.ID) int {
	return slices.IndexFunc(d.Items, func(stored storedItem) bool {
		return stored.ID == string(id)
	})
}

// indexByName returns the position of the item carrying name, or -1.
func (d document) indexByName(name core.Name) int {
	return slices.IndexFunc(d.Items, func(stored storedItem) bool {
		return stored.Name == string(name)
	})
}

// storedItem is the on-disk shape of a single item.
//
// Tags carries no omitempty tag on purpose: an absent tag list and an empty
// one are different states in the domain, and omitempty would write both as
// nothing and read both back as nil.
type storedItem struct {
	// ID is the identifier assigned when the item was created.
	ID string `json:"id"`

	// Name is the item's store-unique name.
	Name string `json:"name"`

	// Description is the item's summary, empty when it has none.
	Description string `json:"description"`

	// Tags are the item's labels. Null and [] mean different things.
	Tags []string `json:"tags"`
}

// newStoredItem converts a domain item into its on-disk form.
func newStoredItem(item core.Item) storedItem {
	var tags []string
	if item.Tags != nil {
		tags = make([]string, len(item.Tags))
		for index, tag := range item.Tags {
			tags[index] = string(tag)
		}
	}

	return storedItem{
		ID:          string(item.ID),
		Name:        string(item.Name),
		Description: string(item.Description),
		Tags:        tags,
	}
}

// item converts the on-disk form back into a domain item.
func (s storedItem) item() core.Item {
	var tags []core.Tag
	if s.Tags != nil {
		tags = make([]core.Tag, len(s.Tags))
		for index, tag := range s.Tags {
			tags[index] = core.Tag(tag)
		}
	}

	return core.Item{
		ID:          core.ID(s.ID),
		Name:        core.Name(s.Name),
		Description: core.Description(s.Description),
		Tags:        tags,
	}
}
