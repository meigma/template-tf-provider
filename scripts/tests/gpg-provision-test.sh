#!/usr/bin/env bash
#
# Tests for scripts/gpg-provision.sh.
#
# The bug that motivated these: the script used to call
# `gh secret set NAME --repo R --body -` believing the dash meant "read stdin".
# It does not. `--body` takes a literal string, and gh reads stdin only when the
# flag is absent, so both release secrets were stored as the one-character value
# "-" and the real key was discarded. Nothing failed until the release workflow
# tried to import the key and gpg answered "usage: gpg [options] [filename]".
#
# The first version of this harness could not catch that, because its fake gh
# always read stdin — the same assumption the script was making. A fake that
# shares the code's misconception tests nothing. So the first case below asserts
# the FAKE's flag handling against the real CLI's documented behavior; if that
# ever drifts, every case after it is worthless and this one goes red first.
#
# Requires gpg (the release path signs with it) and no network access.

set -euo pipefail

# repo_root is the repository this test file lives in.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# provision_script is the script under test.
provision_script="$repo_root/scripts/gpg-provision.sh"

# tests_run and tests_failed drive the final report and exit status.
tests_run=0
tests_failed=0

# workdir holds the fake gh, captured secrets, and throwaway keyrings.
workdir=""

# fail records a failed assertion and keeps going, so one run reports
# everything that is broken rather than only the first thing.
fail() {
	printf '  FAIL: %s\n' "$*" >&2
	tests_failed=$((tests_failed + 1))
}

# pass records a satisfied assertion.
pass() {
	printf '  ok: %s\n' "$*"
}

# expect_equal asserts two strings match.
expect_equal() {
	local description="$1" expected="$2" actual="$3"

	if [ "$expected" = "$actual" ]; then
		pass "$description"
	else
		fail "$description (expected '$expected', got '$actual')"
	fi
}

# start_case announces a test case.
start_case() {
	tests_run=$((tests_run + 1))
	printf '\n[%d] %s\n' "$tests_run" "$1"
}

# cleanup kills any agent started in a throwaway keyring and removes the
# scratch directory.
cleanup() {
	local home
	for home in "$workdir"/gn-*; do
		[ -d "$home" ] || continue
		gpgconf --homedir "$home" --kill all >/dev/null 2>&1 || true
	done
	rm -rf "$workdir"
}

# write_fake_gh creates a gh stand-in that mimics the real CLI's flag handling.
#
# From `gh secret set --help`:
#   -b, --body string   The value for the secret (reads from standard input if
#                       not specified)
# So a body flag always supplies a literal value, and stdin is consumed only
# when no body flag was passed. Reproducing that faithfully is what gives the
# rest of this file its meaning.
write_fake_gh() {
	local path="$1"

	mkdir -p "$(dirname "$path")"
	cat >"$path" <<'FAKE_GH'
#!/usr/bin/env bash
set -euo pipefail

log="${FAKE_GH_LOG:?}"
secret_dir="${FAKE_GH_SECRET_DIR:?}"
existing="${FAKE_GH_EXISTING_SECRETS:-}"

{
	printf 'gh'
	for arg in "$@"; do printf ' %q' "$arg"; done
	printf '\n'
} >>"$log"

command="${1:-} ${2:-}"
shift 2 || true

case "$command" in
"auth status")
	exit 0
	;;
"repo view")
	printf 'meigma/template-tf-provider\n'
	;;
"secret list")
	printf '%s' "$existing" | tr ',' '\n' | grep -v '^$' || true
	;;
"secret set")
	name="${1:?}"
	shift

	body=""
	body_given=false
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--body | -b)
			body="${2:?}"
			body_given=true
			shift 2
			;;
		--body=*)
			body="${1#--body=}"
			body_given=true
			shift
			;;
		-b=*)
			body="${1#-b=}"
			body_given=true
			shift
			;;
		--repo | -R)
			shift 2
			;;
		*)
			shift
			;;
		esac
	done

	if [ "$body_given" = false ]; then
		body="$(cat)"
	fi

	printf '%s' "$body" >"$secret_dir/$name"
	;;
*)
	printf 'fake gh: unhandled command: %s\n' "$command" >&2
	exit 99
	;;
esac
FAKE_GH
	chmod +x "$path"
}

# reset_capture gives a case a clean log and secret directory.
reset_capture() {
	local name="$1"

	export FAKE_GH_LOG="$workdir/$name.log"
	export FAKE_GH_SECRET_DIR="$workdir/$name-secrets"

	: >"$FAKE_GH_LOG"
	rm -rf "$FAKE_GH_SECRET_DIR"
	mkdir -p "$FAKE_GH_SECRET_DIR"
}

# test_fake_gh_matches_real_cli guards the harness itself. If this fails, the
# fake has drifted from the real CLI and the other cases prove nothing.
test_fake_gh_matches_real_cli() {
	start_case "the fake gh handles --body the way the real CLI does"

	reset_capture harness
	export FAKE_GH_EXISTING_SECRETS=""

	printf 'PIPED' | gh secret set NO_FLAG --repo x/y
	expect_equal "no body flag reads stdin" \
		"PIPED" "$(cat "$FAKE_GH_SECRET_DIR/NO_FLAG")"

	printf 'PIPED' | gh secret set DASH --repo x/y --body -
	expect_equal "--body - stores a literal dash, not stdin" \
		"-" "$(cat "$FAKE_GH_SECRET_DIR/DASH")"

	printf 'PIPED' | gh secret set LITERAL --repo x/y --body hunter2
	expect_equal "--body VALUE stores the literal value" \
		"hunter2" "$(cat "$FAKE_GH_SECRET_DIR/LITERAL")"

	printf 'PIPED' | gh secret set JOINED --repo x/y --body=hunter3
	expect_equal "--body=VALUE stores the literal value" \
		"hunter3" "$(cat "$FAKE_GH_SECRET_DIR/JOINED")"
}

# test_stores_real_key asserts the private key reaches gh intact rather than
# being replaced by a flag argument.
test_stores_real_key() {
	start_case "a fresh provision stores the armored private key"

	reset_capture fresh
	export FAKE_GH_EXISTING_SECRETS=""

	"$provision_script" >"$workdir/public.asc" 2>"$workdir/fresh.err"

	local key
	key="$(cat "$FAKE_GH_SECRET_DIR/GPG_PRIVATE_KEY")"

	expect_equal "GPG_PRIVATE_KEY starts with the armor header" \
		"-----BEGIN PGP PRIVATE KEY BLOCK-----" "$(printf '%s' "$key" | head -1)"
	expect_equal "GPG_PRIVATE_KEY ends with the armor footer" \
		"-----END PGP PRIVATE KEY BLOCK-----" "$(printf '%s' "$key" | tail -1)"

	# The exact shape of the regression: a one-character secret.
	if [ "$key" = "-" ]; then
		fail "GPG_PRIVATE_KEY is the literal string '-' (the --body regression is back)"
	else
		pass "GPG_PRIVATE_KEY is not the literal string '-'"
	fi

	if [ "${#key}" -gt 1000 ]; then
		pass "GPG_PRIVATE_KEY is ${#key} bytes, consistent with an RSA 4096 key"
	else
		fail "GPG_PRIVATE_KEY is only ${#key} bytes; that is not an armored private key"
	fi

	local passphrase
	passphrase="$(cat "$FAKE_GH_SECRET_DIR/GPG_PASSPHRASE")"

	if [ "${#passphrase}" -ge 32 ]; then
		pass "GPG_PASSPHRASE is ${#passphrase} bytes"
	else
		fail "GPG_PASSPHRASE is only ${#passphrase} bytes; expected a generated 32-byte value"
	fi

	if grep -q -- "-----BEGIN PGP PUBLIC KEY BLOCK-----" "$workdir/public.asc"; then
		pass "the public key was printed to stdout for registry submission"
	else
		fail "no public key on stdout"
	fi
}

# test_stored_key_signs_and_verifies is the end-to-end claim that matters: what
# was handed to gh is what the release workflow can import and sign with.
test_stored_key_signs_and_verifies() {
	start_case "the stored secrets can import, sign, and verify"

	local signer_home="$workdir/gn-s"
	mkdir -p "$signer_home"
	chmod 700 "$signer_home"

	local key passphrase
	key="$(cat "$workdir/fresh-secrets/GPG_PRIVATE_KEY")"
	passphrase="$(cat "$workdir/fresh-secrets/GPG_PASSPHRASE")"

	# Exactly what release.yml's import step runs.
	if printf '%s' "$key" |
		GNUPGHOME="$signer_home" gpg --batch --quiet --pinentry-mode loopback \
			--passphrase "$passphrase" --import 2>"$workdir/import.err"; then
		pass "the stored private key imports on a headless runner"
	else
		fail "import failed: $(head -1 "$workdir/import.err")"
		return
	fi

	local fingerprint
	fingerprint="$(GNUPGHOME="$signer_home" gpg --list-secret-keys --with-colons |
		awk -F: '/^fpr:/ { print $10; exit }')"

	if [ -z "$fingerprint" ]; then
		fail "no secret key in the keyring after import"
		return
	fi
	pass "imported key $fingerprint"

	# Exactly what .goreleaser.yaml's signs block runs.
	printf 'checksum line\n' >"$workdir/sums"
	if printf '%s' "$passphrase" |
		GNUPGHOME="$signer_home" gpg --batch --pinentry-mode loopback \
			--passphrase-fd 0 --local-user "$fingerprint" \
			--output "$workdir/sums.sig" --detach-sign "$workdir/sums" \
			2>"$workdir/sign.err"; then
		pass "the key produces a detached signature without a tty"
	else
		fail "signing failed: $(head -1 "$workdir/sign.err")"
		return
	fi

	if head -c 30 "$workdir/sums.sig" | LC_ALL=C grep -q -- "-----BEGIN PGP"; then
		fail "the signature is ASCII-armored; the registries require binary"
	else
		pass "the signature is binary, as the registries require"
	fi

	# A consumer holds only the printed public key. --no-autostart keeps gpg from
	# spawning an agent it does not need: verifying touches no secret key.
	local consumer_home="$workdir/gn-c"
	mkdir -p "$consumer_home"
	chmod 700 "$consumer_home"

	if ! GNUPGHOME="$consumer_home" gpg --batch --quiet --no-autostart \
		--import "$workdir/public.asc" 2>"$workdir/consumer-import.err"; then
		fail "the printed public key does not import: $(head -1 "$workdir/consumer-import.err")"
		return
	fi

	if GNUPGHOME="$consumer_home" gpg --no-autostart --verify \
		"$workdir/sums.sig" "$workdir/sums" >/dev/null 2>&1; then
		pass "the printed public key verifies that signature"
	else
		fail "the printed public key does not verify the signature"
	fi
}

# test_refuses_without_force asserts the guard against clobbering a key that
# published releases already depend on.
test_refuses_without_force() {
	start_case "an existing key is not overwritten without --force"

	reset_capture guarded
	export FAKE_GH_EXISTING_SECRETS="GPG_PRIVATE_KEY,GPG_PASSPHRASE,UNRELATED"

	local status=0
	"$provision_script" >/dev/null 2>"$workdir/guarded.err" || status=$?

	expect_equal "exits non-zero" "1" "$status"

	if grep -q -- "--force" "$workdir/guarded.err"; then
		pass "the error explains how to rotate deliberately"
	else
		fail "the error does not mention --force: $(head -1 "$workdir/guarded.err")"
	fi

	local writes
	writes="$(grep -c 'secret set' "$FAKE_GH_LOG" || true)"
	expect_equal "no secret was written" "0" "$writes"
}

# test_force_rotates asserts --force does overwrite, and still stores a real key.
test_force_rotates() {
	start_case "--force rotates the key and stores a real one"

	reset_capture rotated
	export FAKE_GH_EXISTING_SECRETS="GPG_PRIVATE_KEY,GPG_PASSPHRASE"

	"$provision_script" --force --email releases@example.org >/dev/null \
		2>"$workdir/rotated.err"

	expect_equal "GPG_PRIVATE_KEY starts with the armor header" \
		"-----BEGIN PGP PRIVATE KEY BLOCK-----" \
		"$(head -1 "$FAKE_GH_SECRET_DIR/GPG_PRIVATE_KEY")"

	if grep -q "^warning:" "$workdir/rotated.err"; then
		pass "rotation warns about the previously registered key"
	else
		fail "rotation is silent about the old key"
	fi
}

# main sets up the sandbox and runs every case.
main() {
	command -v gpg >/dev/null 2>&1 ||
		{
			printf 'error: gpg is required (the release path signs with it)\n' >&2
			exit 2
		}
	[ -x "$provision_script" ] ||
		{
			printf 'error: %s is missing or not executable\n' "$provision_script" >&2
			exit 2
		}

	# gpg-agent's socket lives inside GNUPGHOME, and a Unix socket path cannot
	# exceed ~104 bytes. macOS TMPDIR paths are long enough that a nested
	# keyring directory under one overflows it, so prefer a short base.
	local tmp_base="/tmp"
	if [ ! -d "$tmp_base" ] || [ ! -w "$tmp_base" ]; then
		tmp_base="${TMPDIR:-/tmp}"
	fi

	workdir="$(mktemp -d "$tmp_base/gpgtest.XXXXXX")"
	trap cleanup EXIT

	write_fake_gh "$workdir/bin/gh"
	export PATH="$workdir/bin:$PATH"

	printf 'Testing %s\n' "$provision_script"

	test_fake_gh_matches_real_cli
	test_stores_real_key
	test_stored_key_signs_and_verifies
	test_refuses_without_force
	test_force_rotates

	printf '\n%d test case(s) run\n' "$tests_run"
	if [ "$tests_failed" -ne 0 ]; then
		printf '%d assertion(s) failed\n' "$tests_failed" >&2
		exit 1
	fi

	printf 'All assertions passed.\n'
}

main "$@"
