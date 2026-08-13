// Command terraform-provider-example serves the example Terraform provider.
//
// Terraform and OpenTofu launch this binary themselves and talk to it over a
// private gRPC channel, so running it from a shell is not useful. Pass -debug
// to keep the provider running under a debugger; it then prints the
// TF_REATTACH_PROVIDERS value Terraform needs to connect to it.
package main
