#!/usr/bin/env bash
#
# Collect the registry-contract assets GoReleaser produced into one directory.
#
# GoReleaser writes the zips, SBOMs, SHA256SUMS, and its detached signature into
# dist/ alongside per-target build directories and its own metadata. It does not
# write the manifest copy: `checksum.extra_files` hashes
# terraform-registry-manifest.json under the release name but leaves the file
# where it is, so the copy is made here.
#
# Usage:
#   scripts/stage-release-assets.sh --version X.Y.Z [--dist dist] [--out DIR]
#
# The result is a directory holding exactly what gets uploaded to the release,
# which scripts/check-release-contract.sh then validates.

set -euo pipefail

# PROJECT is the provider name every asset name is built from.
PROJECT="terraform-provider-example"

# MANIFEST is the repository-root registry manifest that becomes an asset.
MANIFEST="terraform-registry-manifest.json"

# version is the release version without a leading v.
version=""

# dist is GoReleaser's output directory.
dist="dist"

# out is the staging directory; empty means "<dist>/release-assets".
out=""

# fail writes an error and exits non-zero.
fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

# parse_args reads the command line into the globals above.
parse_args() {
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--version)
			[ "$#" -ge 2 ] || fail "--version needs a value"
			version="$2"
			shift 2
			;;
		--dist)
			[ "$#" -ge 2 ] || fail "--dist needs a value"
			dist="$2"
			shift 2
			;;
		--out)
			[ "$#" -ge 2 ] || fail "--out needs a value"
			out="$2"
			shift 2
			;;
		*)
			fail "unknown argument: $1"
			;;
		esac
	done

	[ -n "$version" ] || fail "--version is required"
	[ -d "$dist" ] || fail "distribution directory not found: $dist"
	[ -n "$out" ] || out="$dist/release-assets"
}

# main stages every contract asset into the output directory.
main() {
	parse_args "$@"

	rm -rf "$out"
	mkdir -p "$out"

	local prefix="${PROJECT}_${version}"
	local sums="${prefix}_SHA256SUMS"

	[ -f "$dist/$sums" ] || fail "$dist/$sums is missing; did GoReleaser run?"
	[ -f "$dist/$sums.sig" ] || fail "$dist/$sums.sig is missing; did the signing step run?"
	[ -f "$MANIFEST" ] || fail "$MANIFEST is missing from the repository root"

	# The zips and their SBOMs. Every SBOM is listed in SHA256SUMS, so shipping
	# them keeps the checksum file fully verifiable by a consumer.
	local staged=0
	local file
	for file in "$dist/$prefix"_*.zip "$dist/$prefix"_*.zip.sbom.json; do
		[ -f "$file" ] || fail "no files matched $file"
		cp "$file" "$out/"
		staged=$((staged + 1))
	done

	cp "$MANIFEST" "$out/${prefix}_manifest.json"
	cp "$dist/$sums" "$out/"
	cp "$dist/$sums.sig" "$out/"

	printf 'staged %d build artifacts plus manifest, SHA256SUMS, and signature into %s\n' \
		"$staged" "$out"
}

main "$@"
