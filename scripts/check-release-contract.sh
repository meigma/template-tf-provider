#!/usr/bin/env bash
#
# Assert that a staged release directory satisfies the provider registry
# contract. Both the release workflow and its rehearsal run this before anything
# is uploaded or made public, and it is runnable by hand against a local
# `goreleaser release --snapshot` result.
#
# The contract, for version X.Y.Z:
#
#   terraform-provider-example_X.Y.Z_<os>_<arch>.zip   one per supported
#                                                      platform, each holding a
#                                                      single executable named
#                                                      terraform-provider-example_vX.Y.Z
#   terraform-provider-example_X.Y.Z_manifest.json     copy of the repo manifest
#   terraform-provider-example_X.Y.Z_SHA256SUMS        covers every other file
#   terraform-provider-example_X.Y.Z_SHA256SUMS.sig    detached BINARY GPG sig
#
# A registry that cannot satisfy every line of this refuses the version, so this
# script exits non-zero on the first violation and says which one.
#
# Usage:
#   scripts/check-release-contract.sh --version X.Y.Z [--dir DIR] [--gpg-verify]

set -euo pipefail

# PROJECT is the provider name every asset name is built from.
PROJECT="terraform-provider-example"

# PLATFORMS is the exact os_arch set the release must ship, matching the build
# matrix in .goreleaser.yaml. A platform appearing or disappearing here without
# a deliberate config change means the build silently dropped a target.
PLATFORMS=(
	darwin_amd64
	darwin_arm64
	freebsd_386
	freebsd_amd64
	freebsd_arm
	freebsd_arm64
	linux_386
	linux_amd64
	linux_arm
	linux_arm64
	windows_386
	windows_amd64
	windows_arm64
)

# version is the release version without a leading v.
version=""

# dir is the staged asset directory to inspect.
dir="dist/release-assets"

# gpg_verify turns on signature verification, which needs the signing key's
# public half in the caller's keyring.
gpg_verify=false

# failures counts violations so one run reports every problem, not just the
# first. Only a missing prerequisite aborts early.
failures=0

# fail records a contract violation.
fail() {
	printf 'FAIL: %s\n' "$*" >&2
	failures=$((failures + 1))
}

# abort exits immediately for a problem that makes further checks meaningless.
abort() {
	printf 'error: %s\n' "$*" >&2
	exit 2
}

# ok reports a satisfied assertion.
ok() {
	printf 'ok: %s\n' "$*"
}

# parse_args reads the command line into the globals above.
parse_args() {
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--version)
			[ "$#" -ge 2 ] || abort "--version needs a value"
			version="$2"
			shift 2
			;;
		--dir)
			[ "$#" -ge 2 ] || abort "--dir needs a value"
			dir="$2"
			shift 2
			;;
		--gpg-verify)
			gpg_verify=true
			shift
			;;
		*)
			abort "unknown argument: $1"
			;;
		esac
	done

	[ -n "$version" ] || abort "--version is required"
	[ -d "$dir" ] || abort "staged asset directory not found: $dir"
}

# sha256_check verifies a checksum file, using whichever of the two standard
# tools the host has (Linux runners ship sha256sum, macOS ships shasum).
sha256_check() {
	local sums="$1"

	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum -c --quiet "$sums"
	else
		shasum -a 256 -c "$sums" >/dev/null
	fi
}

# check_zips asserts one zip per platform, no extras, each holding exactly one
# file whose name carries the v-prefixed version Terraform's plugin loader
# expects.
check_zips() {
	local prefix="${PROJECT}_${version}"
	local platform

	for platform in "${PLATFORMS[@]}"; do
		local zip="$dir/${prefix}_${platform}.zip"

		if [ ! -f "$zip" ]; then
			fail "missing zip for $platform: $(basename "$zip")"
			continue
		fi

		local entries
		entries="$(unzip -Z1 "$zip")"

		# Windows binaries keep the .exe GoReleaser appends; the plugin loader
		# looks for it there and nowhere else.
		local expected="${PROJECT}_v${version}"
		case "$platform" in
		windows_*) expected="${expected}.exe" ;;
		esac

		if [ "$entries" != "$expected" ]; then
			fail "$(basename "$zip") must contain exactly one file named $expected, found: $(printf '%s' "$entries" | tr '\n' ' ')"
			continue
		fi

		ok "$(basename "$zip") contains $expected"
	done

	local found
	found="$(find "$dir" -maxdepth 1 -name "${prefix}_*.zip" | wc -l | tr -d ' ')"
	if [ "$found" -ne "${#PLATFORMS[@]}" ]; then
		fail "expected ${#PLATFORMS[@]} zips, found $found"
	else
		ok "zip count is ${#PLATFORMS[@]}"
	fi
}

# check_manifest asserts the manifest asset exists, matches the repository copy,
# and advertises the protocol version the provider actually serves.
check_manifest() {
	local asset="$dir/${PROJECT}_${version}_manifest.json"

	if [ ! -f "$asset" ]; then
		fail "missing manifest asset: $(basename "$asset")"
		return
	fi

	if [ -f terraform-registry-manifest.json ] &&
		! cmp -s terraform-registry-manifest.json "$asset"; then
		fail "$(basename "$asset") differs from terraform-registry-manifest.json"
		return
	fi

	if ! jq -e '.metadata.protocol_versions == ["6.0"]' "$asset" >/dev/null; then
		fail "$(basename "$asset") does not advertise protocol 6.0"
		return
	fi

	ok "$(basename "$asset") matches the repository manifest and declares protocol 6.0"
}

# check_sums asserts the checksum file covers every zip and the manifest, and
# that every checksum in it is correct.
check_sums() {
	local sums_name="${PROJECT}_${version}_SHA256SUMS"
	local sums="$dir/$sums_name"

	if [ ! -f "$sums" ]; then
		fail "missing checksum file: $sums_name"
		return
	fi

	local required=("${PROJECT}_${version}_manifest.json")
	local platform
	for platform in "${PLATFORMS[@]}"; do
		required+=("${PROJECT}_${version}_${platform}.zip")
	done

	local name
	for name in "${required[@]}"; do
		if ! awk -v want="$name" \
			'$2 == want && length($1) == 64 && $1 ~ /^[[:xdigit:]]+$/ { found = 1 }
			 END { exit(found ? 0 : 1) }' "$sums"; then
			fail "$sums_name has no valid sha256 line for $name"
		fi
	done

	# Verifying in the staged directory proves the sums match the bytes that
	# will actually be uploaded, not the ones GoReleaser hashed in dist/.
	if (cd "$dir" && sha256_check "$sums_name"); then
		ok "$sums_name verifies against every file it lists"
	else
		fail "$sums_name does not verify against the staged files"
	fi
}

# check_signature asserts the detached signature exists and is binary. The
# registries parse the signature themselves and reject ASCII armor, which is
# exactly what an unthinking `--armor` in the signing args would produce.
check_signature() {
	local sums_name="${PROJECT}_${version}_SHA256SUMS"
	local sig="$dir/${sums_name}.sig"

	if [ ! -f "$sig" ]; then
		fail "missing signature: $(basename "$sig")"
		return
	fi

	if LC_ALL=C head -c 30 "$sig" | LC_ALL=C grep -q -- "-----BEGIN PGP"; then
		fail "$(basename "$sig") is ASCII-armored; the registries require a binary detached signature"
		return
	fi

	ok "$(basename "$sig") is a binary detached signature"

	if [ "$gpg_verify" != true ]; then
		return
	fi

	if gpg --verify "$sig" "$dir/$sums_name" 2>&1 | sed 's/^/    /'; then
		ok "$(basename "$sig") verifies against the signing key"
	else
		fail "$(basename "$sig") does not verify against the signing key"
	fi
}

# main runs every check and reports the total.
main() {
	parse_args "$@"

	command -v jq >/dev/null 2>&1 || abort "jq is not installed"
	command -v unzip >/dev/null 2>&1 || abort "unzip is not installed"

	printf 'Checking the registry contract for %s in %s\n\n' "$version" "$dir"

	check_zips
	check_manifest
	check_sums
	check_signature

	printf '\n'
	if [ "$failures" -ne 0 ]; then
		printf '%d contract violation(s); this release would be rejected by the registries.\n' \
			"$failures" >&2
		exit 1
	fi

	printf 'Registry contract satisfied.\n'
}

main "$@"
