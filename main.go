package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/meigma/terraform-provider-example/internal/provider"
)

// address is the provider's registry source address. Terraform matches it
// against the `required_providers` entry in a configuration.
const address = "registry.terraform.io/meigma/example"

// version is the provider's release version. GoReleaser overwrites it with
// `-X main.version=` at build time; local builds keep the "dev" default.
var version = "dev"

// main serves the provider until Terraform closes the connection.
func main() {
	var debug bool
	flag.BoolVar(
		&debug,
		"debug",
		false,
		"run the provider in debug mode, for use with debuggers such as delve",
	)
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: address,
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
