#!/usr/bin/env bash
#
# Provision the GPG signing key the release pipeline uses to sign SHA256SUMS.
#
# Both provider registries authenticate a release by verifying the detached
# signature over the SHA256SUMS file against a public key you register with them
# once. This script creates that key pair, stores the private half in GitHub
# Actions secrets, and prints the public half for registry submission.
#
# The key is a release artifact, not a person: it carries no personal identity,
# lives only in this repository's secrets, and is rotated by re-running the
# script with --force and re-submitting the new public key to both registries.
#
# Usage:
#   scripts/gpg-provision.sh [--force] [--repo OWNER/NAME] [--email ADDRESS]
#
# Requires: gpg, gh (authenticated with admin rights on the repository).

set -euo pipefail

# PROJECT is the provider name, which also names the signing key's UID.
PROJECT="terraform-provider-example"

# KEY_TYPE and KEY_LENGTH match what both registries accept. RSA 4096 is the
# conservative choice: the HashiCorp registry has accepted RSA keys since it
# opened, while support for newer curves has been uneven.
KEY_TYPE="rsa4096"

# KEY_EXPIRY is deliberately "never". An expired signing key invalidates nothing
# that was already published, but it does silently break the next release and,
# worse, makes previously published releases fail verification for users whose
# gpg refuses expired keys. Rotation is an explicit act (--force), not a
# calendar event nobody is watching.
KEY_EXPIRY="never"

# SECRET_KEY_NAME and SECRET_PASSPHRASE_NAME are the Actions secrets the release
# and dry-run workflows read.
SECRET_KEY_NAME="GPG_PRIVATE_KEY"
SECRET_PASSPHRASE_NAME="GPG_PASSPHRASE"

# force allows overwriting secrets that already exist.
force=false

# repo is the target repository; empty means "whatever gh resolves here".
repo=""

# email is the key's UID address.
email="releases@meigma.dev"

# log writes a progress line to stderr so stdout carries only the public key.
log() {
	printf '%s\n' "$*" >&2
}

# fail writes an error and exits non-zero.
fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

# usage prints the command line synopsis.
usage() {
	sed -n '3,18p' "$0" | sed 's/^# \{0,1\}//'
}

# parse_args reads the command line into the globals above.
parse_args() {
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--force)
			force=true
			shift
			;;
		--repo)
			[ "$#" -ge 2 ] || fail "--repo needs a value"
			repo="$2"
			shift 2
			;;
		--email)
			[ "$#" -ge 2 ] || fail "--email needs a value"
			email="$2"
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1"
			;;
		esac
	done
}

# require_tools checks that gpg and gh are installed and gh is authenticated.
require_tools() {
	command -v gpg >/dev/null 2>&1 || fail "gpg is not installed"
	command -v gh >/dev/null 2>&1 || fail "gh is not installed"

	gh auth status >/dev/null 2>&1 ||
		fail "gh is not authenticated; run 'gh auth login' first"
}

# resolve_repo fills in the repository slug when the caller did not pass one.
resolve_repo() {
	if [ -n "$repo" ]; then
		return
	fi

	repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)" ||
		fail "could not resolve the repository; pass --repo OWNER/NAME"
}

# secret_exists reports whether a secret is already set on the repository.
secret_exists() {
	local name="$1"

	gh secret list --repo "$repo" --json name --jq '.[].name' |
		grep -qx "$name"
}

# check_existing_secrets refuses to clobber a key that releases already depend
# on. Overwriting it would strand every consumer who trusts the current key.
check_existing_secrets() {
	local existing=""

	if secret_exists "$SECRET_KEY_NAME"; then
		existing="$SECRET_KEY_NAME"
	fi

	if secret_exists "$SECRET_PASSPHRASE_NAME"; then
		existing="${existing:+$existing, }$SECRET_PASSPHRASE_NAME"
	fi

	if [ -z "$existing" ]; then
		return
	fi

	if [ "$force" = true ]; then
		log "warning: overwriting existing secrets ($existing) because --force was given"
		log "warning: releases signed with the old key keep verifying only if you"
		log "         leave its public key registered with both registries"
		return
	fi

	fail "$repo already has $existing; re-run with --force to rotate the signing key"
}

# main generates the key, uploads the secrets, and prints the public key.
main() {
	parse_args "$@"
	require_tools
	resolve_repo

	log "Repository: $repo"

	check_existing_secrets

	# An isolated GNUPGHOME keeps the release key out of the operator's personal
	# keyring. Short path: gpg-agent's socket lives here and Unix socket paths
	# are limited to ~104 bytes, which a temp dir under $TMPDIR can exceed.
	local gnupghome
	gnupghome="$(mktemp -d "${TMPDIR:-/tmp}/gpgprov.XXXXXX")"
	chmod 700 "$gnupghome"

	# shellcheck disable=SC2064 # expand gnupghome now, while it is still set.
	trap "gpgconf --homedir '$gnupghome' --kill all >/dev/null 2>&1 || true; rm -rf '$gnupghome'" EXIT

	export GNUPGHOME="$gnupghome"

	local passphrase
	passphrase="$(gpg --gen-random --armor 2 32)"

	local uid="$PROJECT releases <$email>"

	log "Generating $KEY_TYPE signing key: $uid"
	gpg --batch --quiet \
		--passphrase "$passphrase" \
		--quick-generate-key "$uid" "$KEY_TYPE" sign "$KEY_EXPIRY"

	local fingerprint
	fingerprint="$(gpg --list-secret-keys --with-colons |
		awk -F: '/^fpr:/ { print $10; exit }')"
	[ -n "$fingerprint" ] || fail "could not read the new key's fingerprint"

	log "Fingerprint: $fingerprint"

	# --pinentry-mode loopback is what lets the export run without a terminal to
	# prompt on, matching how the workflows import the key.
	local private_key
	private_key="$(gpg --batch --quiet --pinentry-mode loopback \
		--passphrase "$passphrase" \
		--armor --export-secret-keys "$fingerprint")"

	local public_key
	public_key="$(gpg --armor --export "$fingerprint")"

	log "Setting $SECRET_KEY_NAME on $repo"
	printf '%s' "$private_key" |
		gh secret set "$SECRET_KEY_NAME" --repo "$repo" --body -

	log "Setting $SECRET_PASSPHRASE_NAME on $repo"
	printf '%s' "$passphrase" |
		gh secret set "$SECRET_PASSPHRASE_NAME" --repo "$repo" --body -

	log ""
	log "Done. The private key exists only in $repo's Actions secrets now;"
	log "the copy in $gnupghome is deleted when this script exits."
	log ""
	log "Next steps — register the PUBLIC key below with both registries:"
	log ""
	log "  Terraform Registry: https://registry.terraform.io -> User or Org"
	log "                      Settings -> Signing Keys -> New GPG Key."
	log "                      Do this BEFORE publishing the provider."
	log "  OpenTofu Registry:  open a submission issue at"
	log "                      https://github.com/opentofu/registry/issues/new/choose"
	log "                      and paste the same public key."
	log ""
	log "Fingerprint to record in both places: $fingerprint"
	log ""

	printf '%s\n' "$public_key"
}

main "$@"
