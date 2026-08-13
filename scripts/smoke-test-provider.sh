#!/usr/bin/env bash
#
# Unpack the host platform's release zip and prove the binary inside it runs.
#
# A provider is a plugin, not a command: started outside Terraform it must refuse
# to serve gRPC on the shell's stdio and exit non-zero. That refusal is the
# strongest signal a release job can get cheaply — the binary is executable, it
# is built for this platform, and it reached the plugin handshake.
#
# Usage:
#   scripts/smoke-test-provider.sh --version X.Y.Z [--dir dist/release-assets]

set -euo pipefail

# PROJECT is the provider name every asset name is built from.
PROJECT="terraform-provider-example"

# version is the release version without a leading v.
version=""

# dir is the staged asset directory holding the zips.
dir="dist/release-assets"

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
		--dir)
			[ "$#" -ge 2 ] || fail "--dir needs a value"
			dir="$2"
			shift 2
			;;
		*)
			fail "unknown argument: $1"
			;;
		esac
	done

	[ -n "$version" ] || fail "--version is required"
	[ -d "$dir" ] || fail "staged asset directory not found: $dir"
}

# main unpacks the host zip and runs the provider binary from it.
main() {
	parse_args "$@"

	local host_os host_arch
	host_os="$(go env GOHOSTOS)"
	host_arch="$(go env GOHOSTARCH)"

	local zip="$dir/${PROJECT}_${version}_${host_os}_${host_arch}.zip"
	[ -f "$zip" ] || fail "no release zip for this host ($host_os/$host_arch): $zip"

	local workdir
	workdir="$(mktemp -d)"
	# shellcheck disable=SC2064 # expand workdir now, while it is still set.
	trap "rm -rf '$workdir'" EXIT

	unzip -q "$zip" -d "$workdir"

	local binary="$workdir/${PROJECT}_v${version}"
	if [ "$host_os" = "windows" ]; then
		binary="${binary}.exe"
	fi

	[ -f "$binary" ] || fail "zip does not contain ${PROJECT}_v${version}"
	[ -x "$binary" ] || fail "${PROJECT}_v${version} is not executable"

	printf 'Running %s from %s\n' "$(basename "$binary")" "$(basename "$zip")"

	local output
	if output="$("$binary" 2>&1)"; then
		printf '%s\n' "$output"
		fail "the provider binary exited 0 when run outside Terraform; it should refuse to start"
	fi

	printf '%s\n' "$output"
	printf 'ok: the provider refused to run outside Terraform, as a plugin must\n'
}

main "$@"
