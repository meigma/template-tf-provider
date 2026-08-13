package core

import (
	"slices"
	"strconv"
	"strings"
)

const (
	// MaxNameLength is the longest permitted item name, in bytes.
	MaxNameLength = 64

	// MaxDescriptionLength is the longest permitted item description, in bytes.
	MaxDescriptionLength = 256

	// MaxTagLength is the longest permitted single tag, in bytes.
	MaxTagLength = 32

	// MaxTags is the largest number of tags one item may carry.
	MaxTags = 16
)

// ID uniquely identifies an item inside a [Store]. The store assigns it during
// [Store.Create]; callers never supply one.
type ID string

// Name is the identifier a person gives an item. It is unique within a store
// and is what the example_item data source looks items up by.
type Name string

// Description is an item's optional one-line summary.
type Description string

// Tag is a single label attached to an item.
type Tag string

// Item is the domain object this provider manages.
type Item struct {
	// ID is the store-assigned identifier. It is empty on an item that has not
	// been stored yet.
	ID ID

	// Name is the item's store-unique name.
	Name Name

	// Description summarizes the item. The empty string means no description.
	Description Description

	// Tags labels the item. A nil slice and an empty slice mean different
	// things: nil means no tags attribute was supplied at all, while an empty
	// slice means an empty collection was supplied. The provider maps that
	// distinction onto Terraform's null-versus-empty-set distinction, so
	// nothing in this package may collapse the two.
	Tags []Tag
}

// NewItem builds a validated item out of user-supplied fields.
//
// NewItem normalizes the item (see [Item.Normalize]) and then validates it,
// failing with a [*ValidationError] that names the offending field. The
// returned item carries no ID; [Store.Create] assigns one.
//
// Example:
//
//	item, err := core.NewItem("web-frontend", "public entry point", []core.Tag{"prod", "edge"})
//	if err != nil {
//		return err
//	}
//	stored, err := store.Create(ctx, item)
func NewItem(name Name, description Description, tags []Tag) (Item, error) {
	item := Item{
		Name:        name,
		Description: description,
		Tags:        tags,
	}.Normalize()

	if err := item.Validate(); err != nil {
		return Item{}, err
	}

	return item, nil
}

// Normalize returns a copy of the item with its tags sorted and deduplicated.
//
// Normalization stops there on purpose. Sorting and deduplication are the only
// changes Terraform treats as no-ops for a set-typed attribute; every other
// rule in this package rejects input rather than repairing it, because a
// provider that rewrites a configured value fails the apply that follows. A
// nil tag slice stays nil so the null-versus-empty distinction survives.
func (i Item) Normalize() Item {
	if len(i.Tags) == 0 {
		return i
	}

	tags := slices.Clone(i.Tags)
	slices.Sort(tags)
	i.Tags = slices.Compact(tags)

	return i
}

// Validate reports the first domain rule the item breaks, as a
// [*ValidationError] naming the field to blame. It returns nil when the item
// is storable.
func (i Item) Validate() error {
	if err := validateName(i.Name); err != nil {
		return err
	}

	if err := validateDescription(i.Description); err != nil {
		return err
	}

	return validateTags(i.Tags)
}

// validateName enforces the item naming rule: a lowercase slug that starts
// with a letter, ends with a letter or digit, and separates words with single
// hyphens or underscores.
func validateName(name Name) error {
	switch {
	case name == "":
		return invalid(FieldName, "must not be empty")
	case len(name) > MaxNameLength:
		return invalid(FieldName, "must be at most "+strconv.Itoa(MaxNameLength)+" characters")
	case !isLowerLetter(name[0]):
		return invalid(FieldName, "must start with a lowercase letter (a-z)")
	}

	if reason := checkSlugBody(string(name), "-_"); reason != "" {
		return invalid(FieldName, reason)
	}

	return nil
}

// validateDescription enforces the description length limit and rejects
// surrounding whitespace, which the provider cannot trim away silently.
func validateDescription(description Description) error {
	switch {
	case len(description) > MaxDescriptionLength:
		return invalid(
			FieldDescription,
			"must be at most "+strconv.Itoa(MaxDescriptionLength)+" characters",
		)
	case hasSurroundingSpace(string(description)):
		return invalid(FieldDescription, "must not start or end with whitespace")
	}

	return nil
}

// validateTags enforces the per-item tag budget and the tag naming rule: a
// lowercase slug that starts and ends with a letter or digit and separates
// words with single hyphens.
func validateTags(tags []Tag) error {
	if len(tags) > MaxTags {
		return invalid(FieldTags, "must hold at most "+strconv.Itoa(MaxTags)+" tags")
	}

	for _, tag := range tags {
		switch {
		case tag == "":
			return invalid(FieldTags, "must not contain an empty tag")
		case len(tag) > MaxTagLength:
			return invalid(
				FieldTags,
				"tag "+strconv.Quote(string(tag))+" must be at most "+
					strconv.Itoa(MaxTagLength)+" characters",
			)
		case !isLowerAlphaNum(tag[0]):
			return invalid(
				FieldTags,
				"tag "+strconv.Quote(string(tag))+" must start with a lowercase letter or digit",
			)
		}

		if reason := checkSlugBody(string(tag), "-"); reason != "" {
			return invalid(FieldTags, "tag "+strconv.Quote(string(tag))+" "+reason)
		}
	}

	return nil
}

// checkSlugBody reports why value is not a slug, or the empty string when it
// is one. It assumes the first byte has already been checked and verifies that
// every byte is a lowercase alphanumeric or one of separators, that no two
// separators are adjacent, and that the value does not end in a separator.
func checkSlugBody(value, separators string) string {
	previousWasSeparator := false

	for index := range len(value) {
		char := value[index]

		switch {
		case isLowerAlphaNum(char):
			previousWasSeparator = false
		case strings.IndexByte(separators, char) >= 0:
			if previousWasSeparator {
				return "must not repeat " + strconv.Quote(separators) + " characters"
			}
			previousWasSeparator = true
		default:
			return "must use only lowercase letters, digits, and " + strconv.Quote(separators)
		}
	}

	if previousWasSeparator {
		return "must end with a lowercase letter or digit"
	}

	return ""
}

// isLowerLetter reports whether char is an ASCII lowercase letter.
func isLowerLetter(char byte) bool {
	return char >= 'a' && char <= 'z'
}

// isLowerAlphaNum reports whether char is an ASCII lowercase letter or digit.
func isLowerAlphaNum(char byte) bool {
	return isLowerLetter(char) || (char >= '0' && char <= '9')
}

// hasSurroundingSpace reports whether value starts or ends with whitespace.
func hasSurroundingSpace(value string) bool {
	return strings.TrimSpace(value) != value
}
