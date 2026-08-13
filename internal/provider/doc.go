// Package provider implements the example Terraform provider.
//
// The provider is the plugin's root object: Terraform asks it for its type
// name, its configuration schema, and the resources and data sources it
// offers. [New] returns the constructor that main passes to the plugin server.
package provider
