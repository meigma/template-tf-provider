---
page_title: "How to run the acceptance tests"
description: |-
  Drive the provider through real plan, apply, and destroy cycles, locally or from the Actions tab.
---

# How to run the acceptance tests

The acceptance tests start a real Terraform-compatible CLI and put the provider
through plan, apply, and destroy. Nothing runs them for you: they are not in
`moon ci`, not on pull requests, and not on a schedule. Run them when you
change resource or data source behavior.

## Run them locally

```sh
moon run root:testacc
```

That runs the suite against the pinned OpenTofu. To use the pinned Terraform
instead:

```sh
TF_ACC_TERRAFORM_PATH=$(mise which terraform) moon run root:testacc
```

`TF_ACC_TERRAFORM_PATH` is how the test framework finds a CLI. Left unset, it
looks for a binary named `terraform` and downloads one when it finds none, so
the Moon task fills it in with the pinned `tofu` rather than letting that
happen.

The tests write to temporary directories and clean up after themselves. They
need no credentials, because the provider's store is a local JSON file.

## Run them in CI

Dispatch the **Acceptance Tests** workflow from the repository's Actions tab.
It takes one input, `cli`, with three choices:

| Choice      | Runs against                    |
|-------------|---------------------------------|
| `tofu`      | The pinned OpenTofu (default)   |
| `terraform` | The pinned Terraform            |
| `both`      | Both, as parallel matrix legs   |

With `both`, one leg failing does not cancel the other — which of the two CLIs
broke is the useful part of the result.

## Add a test

Acceptance tests live in `internal/provider/acceptance_test.go` and are named
`TestAcc…`; the Moon task selects them with `-run TestAcc`. The unit tests in
the same package run against a generated mock store and stay in `moon ci`.
