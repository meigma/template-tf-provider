// Package core holds the provider's domain: the [Item] type, the rules an item
// must satisfy, and the [Store] port that item persistence is reached through.
//
// This package is the inside of the hexagon. It imports no framework and
// performs no I/O, so every rule in it runs and tests without side effects.
// Adapters live outside it: internal/client implements [Store] against a JSON
// file, and internal/provider translates Terraform configuration into these
// types and back.
//
// # Validating instead of normalizing
//
// A Terraform provider may not silently rewrite a value the user configured.
// Terraform compares the planned value with the value the provider returns and
// fails the apply when they differ, so trimming whitespace or lowercasing a
// name would break every configuration that relied on it. The rules here
// therefore reject input they dislike and explain why, and [Item.Normalize] is
// limited to changes Terraform treats as no-ops for a set-typed attribute.
package core
