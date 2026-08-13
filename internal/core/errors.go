package core

import "errors"

// ErrNotFound reports that a [Store] holds no item matching the lookup. Store
// implementations wrap it so callers can test for it with [errors.Is].
var ErrNotFound = errors.New("item not found")

// ErrExists reports that a [Store] already holds an item under the requested
// name. Store implementations wrap it so callers can test for it with
// [errors.Is].
var ErrExists = errors.New("item name already in use")

// Field names the part of an [Item] a validation failure belongs to. The
// provider maps it onto a Terraform attribute path so the diagnostic points at
// the offending line of configuration rather than at the resource as a whole.
type Field string

const (
	// FieldName attributes a failure to an item's name.
	FieldName Field = "name"

	// FieldDescription attributes a failure to an item's description.
	FieldDescription Field = "description"

	// FieldTags attributes a failure to an item's tags.
	FieldTags Field = "tags"
)

// ValidationError reports that an [Item] field broke a domain rule.
//
// [Item.Validate] and [NewItem] always fail with this type, so a caller that
// needs to know which field to blame can recover it with [errors.As]:
//
//	var invalid *core.ValidationError
//	if errors.As(err, &invalid) {
//		diags.AddAttributeError(path.Root(string(invalid.Field)), "Invalid item", invalid.Reason)
//	}
type ValidationError struct {
	// Field is the item field the failure belongs to.
	Field Field

	// Reason explains which rule was broken, phrased for the person who wrote
	// the configuration rather than for a maintainer of this package.
	Reason string
}

// Error returns the field name followed by the reason it was rejected.
func (e *ValidationError) Error() string {
	return string(e.Field) + ": " + e.Reason
}

// invalid builds a [*ValidationError] for the given field.
func invalid(field Field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
