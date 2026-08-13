---
page_title: "Tutorial: your first example_item"
description: |-
  Build the provider from source and use it to create, read, and destroy an item.
---

# Tutorial: your first example_item

In this tutorial we compile the provider ourselves, tell OpenTofu to use our
build instead of one from a registry, and then create, read, and destroy an
item with it.

Everything happens in a scratch directory. The only thing the provider writes
is one JSON file that we choose, so nothing outside that directory changes.

## Prerequisites

- A clone of this repository.
- [mise](https://mise.jdx.dev), activated in your shell. Run `mise install`
  in the clone once. It provisions Go, Moon, and OpenTofu, which is everything
  this tutorial needs.

## Step 1: Build the provider

From the repository root:

```sh
moon run root:build
```

Moon compiles the provider and writes one binary:

```
bin/terraform-provider-example
```

That file is the provider. OpenTofu will launch it for us in a moment.

## Step 2: Make a scratch directory

```sh
mkdir ~/example-item-tutorial
```

We will keep our configuration, our state, and the provider's store file there.

## Step 3: Point OpenTofu at our build

OpenTofu normally downloads providers from a registry. A `dev_overrides` block
tells it to run a binary from a directory on disk instead.

Run this **from the repository root**, so that `$PWD/bin` expands to the
directory we just built into:

```sh
cat > ~/example-item-tutorial/dev.tfrc <<EOF
provider_installation {
  dev_overrides {
    "meigma/example" = "$PWD/bin"
  }
  direct {}
}
EOF
```

Now switch to the scratch directory and tell OpenTofu to read that file:

```sh
cd ~/example-item-tutorial
export TF_CLI_CONFIG_FILE=~/example-item-tutorial/dev.tfrc
```

## Step 4: Write a configuration

Create `main.tf` in the scratch directory:

```terraform
terraform {
  required_providers {
    example = {
      source = "meigma/example"
    }
  }
}

provider "example" {
  store_path = "items.json"
}

resource "example_item" "web_frontend" {
  name        = "web-frontend"
  description = "Public entry point for the web tier"
  tags        = ["edge", "prod"]
}

output "web_frontend_id" {
  value = example_item.web_frontend.id
}
```

`store_path` is the file the provider keeps items in. It does not exist yet;
the provider creates it on the first write.

## Step 5: Apply

```sh
tofu apply -auto-approve
```

We do not run `tofu init` first. An overridden provider is never installed, so
there is nothing for `init` to fetch.

OpenTofu warns us that an override is in effect, shows the plan, and then
applies it:

```
Warning: Provider development overrides are in effect
...
example_item.web_frontend: Creating...
example_item.web_frontend: Creation complete after 0s [id=itm-7t3osxx454sx3fd2vp5l27nwag]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

Outputs:

web_frontend_id = "itm-7t3osxx454sx3fd2vp5l27nwag"
```

Your identifier will differ from this one. The store assigns a fresh identifier
to every item it creates.

## Step 6: Look at what the provider wrote

```sh
cat items.json
```

```json
{
  "items": [
    {
      "id": "itm-7t3osxx454sx3fd2vp5l27nwag",
      "name": "web-frontend",
      "description": "Public entry point for the web tier",
      "tags": [
        "edge",
        "prod"
      ]
    }
  ]
}
```

This file is the provider's whole world. A real provider would have made an API
call here instead.

## Step 7: Read the item back

The provider can also look an item up by name. Add this to the end of
`main.tf`:

```terraform
data "example_item" "lookup" {
  name = "web-frontend"
}

output "lookup_tags" {
  value = data.example_item.lookup.tags
}
```

Apply again:

```sh
tofu apply -auto-approve
```

```
Apply complete! Resources: 0 added, 0 changed, 0 destroyed.

Outputs:

lookup_tags = toset([
  "edge",
  "prod",
])
web_frontend_id = "itm-7t3osxx454sx3fd2vp5l27nwag"
```

Notice the resource count: nothing was added or changed. The data source read
the item the resource had already created, and the tags came back sorted.

## Step 8: Destroy

```sh
tofu destroy -auto-approve
```

```
example_item.web_frontend: Destroying... [id=itm-7t3osxx454sx3fd2vp5l27nwag]
example_item.web_frontend: Destruction complete after 0s

Destroy complete! Resources: 1 destroyed.
```

Check the store file one last time:

```sh
cat items.json
```

```json
{
  "items": []
}
```

The item is gone and the file remains. Delete the scratch directory whenever
you like:

```sh
cd ~ && rm -rf ~/example-item-tutorial
```

## What we did

- Compiled the provider with `moon run root:build`.
- Ran our own build through a `dev_overrides` block and `TF_CLI_CONFIG_FILE`.
- Created an item, read it back through a data source, and destroyed it, and
  watched the provider's store change at each step.

The same override setup is how you try any change you make to this provider.
[Use the provider from a local build](../how-to/local-build.md) covers it on
its own, including how to turn it off again. For what the resource and data
source accept, see the [`example_item` resource](../resources/item.md) and the
[`example_item` data source](../data-sources/item.md). For why the provider is
laid out the way it is, see [How this provider is
structured](../explanation/architecture.md).
