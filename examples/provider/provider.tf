terraform {
  required_providers {
    example = {
      source = "meigma/example"
    }
  }
}

provider "example" {
  # Where the items are kept. May also be set with the EXAMPLE_STORE_PATH
  # environment variable. The file and its parent directory are created on the
  # first write.
  store_path = "/var/lib/example/items.json"
}
