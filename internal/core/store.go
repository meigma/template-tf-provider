package core

import "context"

// Store is the port item persistence is reached through.
//
// The provider depends on this interface and never on a concrete backend,
// which is what lets the resource and data source be tested against a
// generated mock while internal/client is tested on its own against a real
// file. An implementation is an adapter: it translates these five operations
// onto whatever it stores items in.
//
// Implementations report the two conditions callers branch on by wrapping
// [ErrNotFound] and [ErrExists]. Everything else is an opaque failure.
type Store interface {
	// Create stores item under a freshly assigned [ID] and returns the stored
	// copy. It fails with an error wrapping [ErrExists] when another item
	// already holds the same name.
	Create(ctx context.Context, item Item) (Item, error)

	// Get returns the item carrying id, or an error wrapping [ErrNotFound].
	Get(ctx context.Context, id ID) (Item, error)

	// GetByName returns the item carrying name, or an error wrapping
	// [ErrNotFound]. It backs the example_item data source.
	GetByName(ctx context.Context, name Name) (Item, error)

	// Update replaces the stored item that carries item.ID and returns the
	// stored copy. It fails with an error wrapping [ErrNotFound] when no item
	// carries that ID, and one wrapping [ErrExists] when the new name is
	// already held by a different item.
	Update(ctx context.Context, item Item) (Item, error)

	// Delete removes the item carrying id. It fails with an error wrapping
	// [ErrNotFound] when no item carries that ID.
	Delete(ctx context.Context, id ID) error
}
