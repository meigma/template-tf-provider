// Package provider adapts Terraform to the example provider's domain.
//
// It is the outside of the hexagon on the driving side: everything here exists
// to translate Terraform's configuration, plan, and state values into
// internal/core types and to turn the domain's failures back into diagnostics
// pointed at the attribute that caused them. No rule about what makes an item
// valid lives in this package.
//
// [New] returns the constructor main passes to the plugin server. Configure
// builds the store from the provider block and hands it to the resource and
// data source as a core.Store, which is the seam the tests replace with a
// generated mock.
package provider
