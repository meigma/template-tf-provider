---
page_title: "How this provider is structured"
description: |-
  Why the provider is split into a domain, a storage adapter, and a Terraform adapter, and why it never rewrites a configured value.
---

# How this provider is structured

The provider is three packages, and the split is the point of the template. If
you only ever read one explanation before replacing `example_item` with a real
resource, read this one.

## Three packages, one direction of dependency

`internal/core` is the domain. It holds the `Item` type, the rules an item must
satisfy, and a `Store` interface. It imports no framework, opens no files, and
makes no network calls, so every rule in it runs and tests without side
effects.

`internal/client` implements `Store` against a JSON file on disk. It knows
nothing about Terraform. A real provider would put an HTTP client here, and the
shape would not change: a constructor that takes connection details, a type
that satisfies the interface, and no Terraform types anywhere in the package.

`internal/provider` is the Terraform side. Everything in it exists to turn
Terraform's configuration, plan, and state values into `core` types, and to
turn the domain's failures back into diagnostics pointed at the attribute that
caused them. No rule about what makes an item valid lives there.

Dependencies point inward: both adapters import `core`, and `core` imports
neither. That is what makes `Store` a seam rather than a layer. The provider
depends on the interface, so its resource and data source can be exercised
against a generated mock, while `internal/client` is tested on its own against
a real file. Neither test needs the other half to exist.

The domain also decides what the adapters may branch on. `Store`
implementations report exactly two conditions in a way callers can inspect —
they wrap `ErrNotFound` and `ErrExists`. Everything else is an opaque failure,
which keeps the provider from growing special handling for backend-specific
errors.

## Validating instead of normalizing

The rule that surprises people: **a provider may not silently rewrite a value
the user configured.**

Terraform compares the value it planned with the value the provider returns
after applying, and fails the apply when the two differ. So trimming whitespace
from a name, or lowercasing it, does not tidy the user's configuration — it
breaks every apply that relied on the original spelling, with an error about a
provider producing an inconsistent result.

The rules in `core` therefore reject input they dislike and say why, rather
than repairing it. A name with a capital letter is an error naming the `name`
attribute, not a name quietly converted to lowercase.

`Item.Normalize` is the one exception, and it is deliberately tiny: it sorts
and deduplicates tags. Those are the only changes Terraform treats as no-ops,
because `tags` is a set, and a set has no order and no duplicates. Nothing else
belongs in that method.

Two consequences follow from the same rule and are easy to miss:

- `Create` saves the plan with the assigned identifier filled in, rather than a
  model rebuilt from the store. Rebuilding would risk returning a value that
  differs from the planned one; the domain guarantees the two agree precisely
  because it refuses to rewrite anything.
- A nil tag slice and an empty tag slice mean different things and must stay
  distinct all the way through. They map onto Terraform's difference between a
  null attribute and an empty set, and collapsing them in the domain would show
  up as a permanent diff.

## Where a validation failure surfaces

A broken rule fails with a `ValidationError` carrying the field to blame.
`internal/provider` recovers it and maps that field onto a Terraform attribute
path, so the error points at the offending line of configuration instead of at
the resource as a whole. That mapping is the only reason the domain names
fields at all — it has no other use for the concept, and it is why `core`
exports `Field` values that happen to match the schema's attribute names.

The rules those errors come from are listed with the attributes they apply to
on the [`example_item` resource](../resources/item.md) page.
