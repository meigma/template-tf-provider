package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/meigma/terraform-provider-example/internal/client"
	"github.com/meigma/terraform-provider-example/internal/core"
)

// The acceptance tests are the third testing layer: the unit tests exercise the
// domain, the tests alongside them exercise the resource and data source
// against a mock store, and these run a real Terraform (or OpenTofu) binary
// against the real file store.
//
// They only run when TF_ACC is set, which is what [resource.ParallelTest]
// enforces; `go test ./...` skips them. Run them with `moon run root:testacc`.
// Each test owns a store file under its own temp directory, so they can run in
// parallel without seeing each other's items.

const (
	// providerNamespace is the registry namespace the provider is published
	// under. It has to agree with the address main.go serves at.
	providerNamespace = "meigma"

	// providerSource is the full registry address the test configurations
	// require the provider from.
	providerSource = "registry.terraform.io/" + providerNamespace + "/" + TypeName

	// itemTypeName is the Terraform type both the resource and the data source
	// are registered under.
	itemTypeName = TypeName + "_item"

	// itemResourceAddress is the resource block every acceptance test manages.
	itemResourceAddress = itemTypeName + ".test"

	// itemDataSourceAddress is the data source block the lookup test reads.
	itemDataSourceAddress = "data." + itemTypeName + ".lookup"

	// dataAddressPrefix marks the state addresses that belong to data sources
	// rather than to managed resources.
	dataAddressPrefix = "data."
)

// providerFactories serves the provider inside the test process. Terraform
// reaches it over the plugin protocol exactly as it would a released binary, so
// nothing about the provider is stubbed out here; only the discovery of the
// executable is.
func providerFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		TypeName: providerserver.NewProtocol6WithError(New("acctest")()),
	}
}

// TestMain pins the namespace the running provider is registered under before
// any acceptance test starts a CLI.
//
// Left alone, terraform-plugin-testing registers the provider twice: once under
// "hashicorp" and once under Terraform 0.12's legacy "-" namespace. OpenTofu
// refuses to parse the second address at all — "the legacy provider namespace
// can be used only with hostname registry.opentofu.org" — and fails every
// `init`. Naming the namespace is the library's supported way to collapse the
// pair into the single address the configurations here require, and using the
// provider's real one keeps the tests honest about how it is published.
func TestMain(m *testing.M) {
	if err := os.Setenv(resource.EnvTfAccProviderNamespace, providerNamespace); err != nil {
		fmt.Fprintln(os.Stderr, "setting the acceptance test provider namespace:", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// acceptanceStore is the item store one acceptance test runs against.
type acceptanceStore struct {
	// path is the JSON file the provider under test reads and writes.
	path string
}

// newAcceptanceStore returns a store backed by a file under the test's own
// temporary directory. The file is not created here: the provider creates it on
// the first write, which is part of what the tests prove.
func newAcceptanceStore(t *testing.T) acceptanceStore {
	t.Helper()

	return acceptanceStore{path: filepath.Join(t.TempDir(), "items.json")}
}

// config renders a Terraform configuration: the provider requirement, a
// provider block pointing at this test's store, and then body.
//
// Naming the source explicitly is what lets the same configuration run under
// both CLIs. A bare provider name would be resolved against each CLI's own
// default registry, and only one of those matches the address the in-process
// provider is served at.
func (s acceptanceStore) config(body string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    %[1]s = {
      source = %[2]q
    }
  }
}

provider %[1]q {
  store_path = %[3]q
}
%[4]s`, TypeName, providerSource, s.path, body)
}

// itemConfig renders a configuration holding a single example_item resource.
func (s acceptanceStore) itemConfig(name, description string) string {
	return s.config(fmt.Sprintf(`
resource %[1]q "test" {
  name        = %[2]q
  description = %[3]q
  tags        = ["prod", "edge"]
}
`, itemTypeName, name, description))
}

// lookupConfig renders a configuration holding an example_item resource and a
// data source that looks the same item up by name. Referring to the resource's
// name attribute is what orders the two: Terraform reads the data source only
// after the item exists.
func (s acceptanceStore) lookupConfig(name, description string) string {
	return s.itemConfig(name, description) + fmt.Sprintf(`
data %[1]q "lookup" {
  name = %[1]s.test.name
}
`, itemTypeName)
}

// checkDestroyed fails unless every item Terraform managed is gone from the
// store. Terraform reporting a clean destroy is not enough on its own: the
// point of the check is that Delete reached the backend rather than only
// dropping the resource from state.
//
// The state it is handed is the one from just before the destroy, which is
// where the identifiers to look up come from. Finding none of them means the
// check verified nothing, so that counts as a failure too.
func (s acceptanceStore) checkDestroyed(state *terraform.State) error {
	store := client.New(client.Path(s.path))
	checked := 0

	for address, res := range state.RootModule().Resources {
		if res.Type != itemTypeName || strings.HasPrefix(address, dataAddressPrefix) {
			continue
		}

		checked++

		id := core.ID(res.Primary.Attributes[idAttribute])

		if _, err := store.Get(context.Background(), id); !errors.Is(err, core.ErrNotFound) {
			return fmt.Errorf("%s: item %q outlived the destroy: %w", address, id, err)
		}
	}

	if checked == 0 {
		return errors.New("no example_item resources were in state before the destroy, so nothing was verified")
	}

	return nil
}

// expectedTags is the tag set every acceptance configuration applies. The store
// sorts tags on the way to disk, and the attribute is a set, so neither the
// configured order nor the stored order is what the check compares.
func expectedTags() knownvalue.Check {
	return knownvalue.SetExact([]knownvalue.Check{
		knownvalue.StringExact("edge"),
		knownvalue.StringExact("prod"),
	})
}

// TestAccItemResource drives example_item through a full lifecycle against a
// real CLI: create, rename in place, import, and destroy.
func TestAccItemResource(t *testing.T) {
	store := newAcceptanceStore(t)

	// Collected in both apply steps. The identifier belongs to the store, so a
	// rename that produced a new one would mean the resource was replaced
	// rather than updated, whatever the plan claimed.
	identifier := statecheck.CompareValue(compare.ValuesSame())

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories(),
		CheckDestroy:             store.checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: store.itemConfig("acc-item", "the original description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						itemResourceAddress,
						tfjsonpath.New(idAttribute),
						knownvalue.StringRegexp(regexp.MustCompile(`^itm-[a-z0-9]+$`)),
					),
					statecheck.ExpectKnownValue(
						itemResourceAddress,
						tfjsonpath.New(string(core.FieldName)),
						knownvalue.StringExact("acc-item"),
					),
					statecheck.ExpectKnownValue(
						itemResourceAddress,
						tfjsonpath.New(string(core.FieldDescription)),
						knownvalue.StringExact("the original description"),
					),
					statecheck.ExpectKnownValue(
						itemResourceAddress,
						tfjsonpath.New(string(core.FieldTags)),
						expectedTags(),
					),
					identifier.AddStateValue(itemResourceAddress, tfjsonpath.New(idAttribute)),
				},
			},
			{
				Config: store.itemConfig("acc-item-renamed", "the revised description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(itemResourceAddress, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						itemResourceAddress,
						tfjsonpath.New(string(core.FieldName)),
						knownvalue.StringExact("acc-item-renamed"),
					),
					statecheck.ExpectKnownValue(
						itemResourceAddress,
						tfjsonpath.New(string(core.FieldDescription)),
						knownvalue.StringExact("the revised description"),
					),
					identifier.AddStateValue(itemResourceAddress, tfjsonpath.New(idAttribute)),
				},
			},
			{
				// Imports the renamed item by its store identifier and compares
				// the state Read produced against the state the apply left
				// behind. They have to match attribute for attribute, which is
				// the check that catches a Read that drops or reshapes a value.
				ResourceName:      itemResourceAddress,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccItemDataSource looks an applied item up by name and checks that the
// data source reports the same object the resource created.
func TestAccItemDataSource(t *testing.T) {
	store := newAcceptanceStore(t)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories(),
		CheckDestroy:             store.checkDestroyed,
		Steps: []resource.TestStep{
			{
				Config: store.lookupConfig("acc-lookup", "found by name"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(
						itemResourceAddress,
						tfjsonpath.New(idAttribute),
						itemDataSourceAddress,
						tfjsonpath.New(idAttribute),
						compare.ValuesSame(),
					),
					statecheck.ExpectKnownValue(
						itemDataSourceAddress,
						tfjsonpath.New(string(core.FieldDescription)),
						knownvalue.StringExact("found by name"),
					),
					statecheck.ExpectKnownValue(
						itemDataSourceAddress,
						tfjsonpath.New(string(core.FieldTags)),
						expectedTags(),
					),
				},
			},
		},
	})
}
