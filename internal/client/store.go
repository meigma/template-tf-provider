package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/meigma/terraform-provider-example/internal/core"
)

const (
	// idPrefix marks a generated identifier as one of this store's, so a value
	// that turns up in Terraform state is recognizable on sight.
	idPrefix = "itm-"

	// dirPerm is the mode the store's parent directory is created with.
	dirPerm = 0o700

	// tempPattern names the scratch files the atomic write goes through.
	tempPattern = ".items-*.json.tmp"
)

// Path is the location of the JSON file a [Store] reads and writes.
type Path string

// Compile-time proof that the adapter still satisfies the port it was written
// for. Dropping or renaming a method breaks the build here rather than in the
// provider package.
var _ core.Store = (*Store)(nil)

// Store persists items in a single JSON file.
//
// The zero value is not usable; build one with [New].
type Store struct {
	// path is the file the store reads and writes.
	path Path

	// mu serializes operations so a read-modify-write cycle cannot interleave
	// with another one in this process.
	mu sync.Mutex
}

// New returns a store backed by the JSON file at path.
//
// New touches no files. The parent directory is created, and the file itself
// written, on the first operation that changes something; until then a missing
// file simply reads as an empty store.
//
// Example:
//
//	store := client.New("/var/lib/example/items.json")
//	item, err := store.Create(ctx, core.Item{Name: "web-frontend"})
func New(path Path) *Store {
	return &Store{path: path}
}

// Create stores item under a freshly generated ID and returns the stored copy.
// It fails with an error wrapping [core.ErrExists] when the name is taken.
func (s *Store) Create(_ context.Context, item core.Item) (core.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.load()
	if err != nil {
		return core.Item{}, err
	}

	if doc.indexByName(item.Name) >= 0 {
		return core.Item{}, fmt.Errorf("creating item %q: %w", item.Name, core.ErrExists)
	}

	item.ID = newID()
	doc.Items = append(doc.Items, newStoredItem(item))

	if err := s.save(doc); err != nil {
		return core.Item{}, err
	}

	return item, nil
}

// Get returns the item carrying id, or an error wrapping [core.ErrNotFound].
func (s *Store) Get(_ context.Context, id core.ID) (core.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.load()
	if err != nil {
		return core.Item{}, err
	}

	index := doc.indexByID(id)
	if index < 0 {
		return core.Item{}, fmt.Errorf("reading item %q: %w", id, core.ErrNotFound)
	}

	return doc.Items[index].item(), nil
}

// GetByName returns the item carrying name, or an error wrapping
// [core.ErrNotFound].
func (s *Store) GetByName(_ context.Context, name core.Name) (core.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.load()
	if err != nil {
		return core.Item{}, err
	}

	index := doc.indexByName(name)
	if index < 0 {
		return core.Item{}, fmt.Errorf("reading item named %q: %w", name, core.ErrNotFound)
	}

	return doc.Items[index].item(), nil
}

// Update replaces the stored item carrying item.ID and returns the stored
// copy. It fails with an error wrapping [core.ErrNotFound] when the ID is
// unknown, or [core.ErrExists] when the new name belongs to another item.
func (s *Store) Update(_ context.Context, item core.Item) (core.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.load()
	if err != nil {
		return core.Item{}, err
	}

	index := doc.indexByID(item.ID)
	if index < 0 {
		return core.Item{}, fmt.Errorf("updating item %q: %w", item.ID, core.ErrNotFound)
	}

	if conflict := doc.indexByName(item.Name); conflict >= 0 && conflict != index {
		return core.Item{}, fmt.Errorf("updating item %q: %w", item.ID, core.ErrExists)
	}

	doc.Items[index] = newStoredItem(item)

	if err := s.save(doc); err != nil {
		return core.Item{}, err
	}

	return item, nil
}

// Delete removes the item carrying id. It fails with an error wrapping
// [core.ErrNotFound] when the ID is unknown.
func (s *Store) Delete(_ context.Context, id core.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := s.load()
	if err != nil {
		return err
	}

	index := doc.indexByID(id)
	if index < 0 {
		return fmt.Errorf("deleting item %q: %w", id, core.ErrNotFound)
	}

	doc.Items = slices.Delete(doc.Items, index, index+1)

	return s.save(doc)
}

// load reads and decodes the document. A missing or empty file is an empty
// store rather than an error, which is what lets a fresh configuration apply
// without anyone seeding the file first.
func (s *Store) load() (document, error) {
	data, err := os.ReadFile(string(s.path))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return document{}, nil
	case err != nil:
		return document{}, fmt.Errorf("reading item store %s: %w", s.path, err)
	case len(bytes.TrimSpace(data)) == 0:
		return document{}, nil
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return document{}, fmt.Errorf("decoding item store %s: %w", s.path, err)
	}

	return doc, nil
}

// save writes the document out atomically: the encoded bytes land in a
// temporary file beside the target and are renamed over it, so a reader either
// sees the whole previous document or the whole new one, never a partial write.
func (s *Store) save(doc document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding item store %s: %w", s.path, err)
	}

	dir := filepath.Dir(string(s.path))
	if err = os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("creating item store directory %s: %w", dir, err)
	}

	temp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("creating temporary item store in %s: %w", dir, err)
	}
	defer func() { _ = os.Remove(temp.Name()) }()

	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()

		return fmt.Errorf("writing temporary item store %s: %w", temp.Name(), err)
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing temporary item store %s: %w", temp.Name(), err)
	}

	if err := os.Rename(temp.Name(), string(s.path)); err != nil {
		return fmt.Errorf("replacing item store %s: %w", s.path, err)
	}

	return nil
}

// newID returns a fresh, unguessable item identifier.
func newID() core.ID {
	return core.ID(idPrefix + strings.ToLower(rand.Text()))
}
