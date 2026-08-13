---
page_title: "How to publish the provider to the registries"
description: |-
  Provision the signing key, register it with the Terraform and OpenTofu registries, and list the provider so releases are ingested.
---

# How to publish the provider to the registries

Do this once, before the first release. After it is done, every release the
pipeline publishes is picked up by both registries without further action.

The order matters: neither registry accepts a release it cannot verify against
a public key you registered first, and a release that is rejected once is not
re-ingested when you fix the key later.

## Before you start

- The repository is public and named `terraform-provider-<name>`, all
  lowercase. The Terraform Registry derives the provider name from the
  repository name and does not accept any other pattern.
- You are an admin on the repository, and on the GitHub organization that owns
  it if it is not owned by your personal account.
- `gh` is installed and authenticated as that admin.
- The **Release Dry Run** workflow has passed. It rehearses the whole build,
  including GPG signing and the registry contract check, so a green run means
  the only thing left is registration.

## Provision the signing key

```sh
scripts/gpg-provision.sh
```

The script generates an RSA 4096 signing key, stores the private half in the
`GPG_PRIVATE_KEY` Actions secret and its passphrase in `GPG_PASSPHRASE`, prints
the fingerprint, and prints the ASCII-armored public key on stdout.

Save that public key. You paste the same text into both registries below.

The private half exists only in the repository's Actions secrets; the script
deletes its working copy on exit. To rotate the key later, re-run the script
with `--force` and register the new public key with both registries again.
Without `--force` the script refuses to overwrite secrets that already exist.

## Register the public key with the Terraform Registry

1. Sign in at [registry.terraform.io](https://registry.terraform.io) with the
   GitHub account that has access to the repository.
2. Open **User Settings**, or **Organization Settings** for a provider owned by
   an organization you administer.
3. Open **Signing Keys** and add a new GPG key.
4. Paste the ASCII-armored public key, including the
   `-----BEGIN PGP PUBLIC KEY BLOCK-----` and `-----END …-----` lines.

Register the key under the namespace that will own the provider. A key on your
personal namespace does not verify releases published under an organization.

## Register the public key with the OpenTofu Registry

OpenTofu takes key submissions as GitHub issues on
[`opentofu/registry`](https://github.com/opentofu/registry/issues/new/choose).
Choose **Submit new Provider Signing Key** and fill in the form:

| Field | Value |
|-------|-------|
| Provider Namespace | The GitHub user or organization that owns the repository |
| Provider Name | The name without the `terraform-provider-` prefix, or blank to register the key for the whole namespace |
| Public Membership | Tick the box |
| Provider GPG Key | The same ASCII-armored public key |

Two constraints are easy to trip over:

- If the provider belongs to an organization, your membership in that
  organization must be **publicly visible** before you submit. Automated
  validation checks it and fails the submission otherwise. Change it under
  GitHub's organization membership settings.
- Submit through the issue form in a browser. Creating the issue with `gh` or
  the API produces a body the registry's automation cannot read, and the
  submission is not processed.

## List the provider

The Terraform Registry needs one manual publish; after that a webhook on the
repository reports new tags.

1. In the Terraform Registry, choose **Publish** and then **Provider** from the
   top-right menu.
2. Select the organization and the repository.
3. Confirm. The registry creates the release webhook on the repository.

For OpenTofu, open a second issue on `opentofu/registry` using the **Submit new
Provider** form and give the repository as `{owner}/terraform-provider-{name}`.
An OpenTofu team member reviews it.

## What happens on each release afterwards

Nothing manual. Merging a Release Please pull request creates the tag and a
draft GitHub release, and the release workflow fills that draft with the assets
both registries read:

- one zip per platform, each holding a single binary named
  `terraform-provider-example_vX.Y.Z`
- `terraform-provider-example_X.Y.Z_manifest.json`
- `terraform-provider-example_X.Y.Z_SHA256SUMS`
- `terraform-provider-example_X.Y.Z_SHA256SUMS.sig`, the detached GPG signature
  the registries verify against your registered key

The workflow also attaches a CycloneDX SBOM per zip, a cosign bundle over the
checksum file, and GitHub build provenance. Both registries ignore those; they
are there for consumers who want them.

The release stays a draft until every job succeeds. A failure anywhere leaves
it a draft, which both registries treat as not existing — so a broken release
is never ingested.

## Verify a published release

Give these to anyone who wants to check an artifact themselves. Import the
project's public signing key first, then:

```sh
gh release download vX.Y.Z --repo meigma/template-tf-provider --dir provider
cd provider

# 1. The signature the registries themselves check.
gpg --verify terraform-provider-example_X.Y.Z_SHA256SUMS.sig \
  terraform-provider-example_X.Y.Z_SHA256SUMS

# 2. Every downloaded file matches the checksum file.
sha256sum -c terraform-provider-example_X.Y.Z_SHA256SUMS

# 3. The checksum file came from this repository's release workflow.
cosign verify-blob \
  --bundle terraform-provider-example_X.Y.Z_SHA256SUMS.sigstore.json \
  --certificate-identity https://github.com/meigma/template-tf-provider/.github/workflows/release.yml@refs/tags/vX.Y.Z \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  terraform-provider-example_X.Y.Z_SHA256SUMS

# 4. GitHub-native build provenance for any artifact.
gh attestation verify terraform-provider-example_X.Y.Z_linux_amd64.zip \
  --repo meigma/template-tf-provider \
  --signer-workflow meigma/template-tf-provider/.github/workflows/attest.yml \
  --source-ref refs/tags/vX.Y.Z \
  --deny-self-hosted-runners
```

Replace `terraform-provider-example` and `meigma/template-tf-provider` with the
provider name and repository slug of the project you are publishing.
