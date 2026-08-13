---
page_title: "How to use the provider from a local build"
description: |-
  Run a provider binary you compiled yourself instead of one installed from a registry.
---

# How to use the provider from a local build

Use this when you want a `tofu` or `terraform` run to exercise your working
copy of the provider rather than a published release. If you have not done it
before, [Your first example_item](../tutorials/first-item.md) walks the same
setup one step at a time.

## Build the binary

```sh
moon run root:build
```

The binary lands at `bin/terraform-provider-example`. Rebuild after every
change you want to see; nothing rebuilds it for you.

## Write a CLI configuration file

A `dev_overrides` block maps a provider source address to a *directory*
containing the binary, not to the binary itself. Run this from the repository
root so `$PWD/bin` expands to an absolute path:

```sh
cat > ~/.example-provider.tfrc <<EOF
provider_installation {
  dev_overrides {
    "meigma/example" = "$PWD/bin"
  }
  direct {}
}
EOF
```

Keep the `direct {}` block. Without it, every *other* provider in the
configuration also stops being installable.

The address on the left is the one your configuration asks for in
`required_providers`. If you renamed the provider, change it here too.

## Point the CLI at that file

```sh
export TF_CLI_CONFIG_FILE=~/.example-provider.tfrc
```

The same variable and the same file work for both `tofu` and `terraform`.

To make the override permanent for one CLI instead, put the
`provider_installation` block in that CLI's own configuration file
(`~/.terraformrc` for Terraform, `~/.tofurc` for OpenTofu) and leave
`TF_CLI_CONFIG_FILE` unset.

## Run without `init`

Do not run `init` while an override is in effect:

```
Skip tofu init when using provider development overrides. It is not necessary
and may error unexpectedly.

Error: Failed to query available provider packages
```

An overridden provider is never installed, so there is nothing to fetch and no
lock file entry to write. Go straight to `plan`, `apply`, or `destroy`. Every
command prints a warning naming the overridden address and the directory it is
being loaded from — check that line when a change you made does not seem to
take effect.

If the configuration also uses providers that are *not* overridden, run `init`
once before adding the override, then work with the override in place.

## Turn it off

```sh
unset TF_CLI_CONFIG_FILE
```

State written under an override stays on disk. It was produced by an unreleased
build, so treat it as scratch state: destroy it, or delete the state file, before
pointing the same directory at a released provider.

## Attach a debugger instead

`moon run root:build` produces a normal binary. To step through provider code,
run it yourself with `-debug` and export the `TF_REATTACH_PROVIDERS` value it
prints; the CLI then connects to your running process and no override is
needed.
