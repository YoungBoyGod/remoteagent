#!/usr/bin/env bash
set -euo pipefail

# test-distribute.sh — End-to-end integration tests for SecureRelease distribution system
# Tests: secure-distribute.sh encrypt/decrypt/verify + generate-decrypt-guide.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SECURE_DIST="${SCRIPT_DIR}/secure-distribute.sh"
DECRYPT_GUIDE="${SCRIPT_DIR}/generate-decrypt-guide.sh"

PASSED=0
FAILED=0
TMPDIR=""

# --- Setup / Teardown ---

setup() {
    TMPDIR="$(mktemp -d /tmp/test-distribute.XXXXXX)"
}

cleanup() {
    if [[ -n "$TMPDIR" && -d "$TMPDIR" ]]; then
        rm -rf "$TMPDIR"
    fi
}
trap cleanup EXIT INT TERM

# --- Test helpers ---

pass() {
    PASSED=$((PASSED + 1))
    echo "  PASS: $1"
}

fail() {
    FAILED=$((FAILED + 1))
    echo "  FAIL: $1"
    if [[ -n "${2:-}" ]]; then
        echo "        $2"
    fi
}

run_test() {
    local name="$1"
    echo "--- $name ---"
}

# --- Tests ---

test_encrypt_decrypt_sha256() {
    run_test "encrypt -> decrypt -> SHA-256 roundtrip (1KB file)"

    local input="${TMPDIR}/testfile.bin"
    local encrypted="${TMPDIR}/testfile.bin.gpg"
    local decrypted="${TMPDIR}/testfile.decrypted.bin"
    local keyfile="${TMPDIR}/session.key"

    # Generate 1KB random file
    dd if=/dev/urandom of="$input" bs=1024 count=1 2>/dev/null

    local sha_before
    sha_before="$(sha256sum "$input" | awk '{print $1}')"

    # Encrypt
    if ! bash "$SECURE_DIST" \
        --action encrypt \
        --input "$input" \
        --output "$encrypted" \
        --password-file "$keyfile" 2>/dev/null; then
        fail "encrypt command failed"
        return
    fi

    if [[ ! -f "$encrypted" ]]; then
        fail "encrypted file not created"
        return
    fi

    if [[ ! -f "$keyfile" ]]; then
        fail "session key file not created"
        return
    fi

    # Decrypt
    if ! bash "$SECURE_DIST" \
        --action decrypt \
        --input "$encrypted" \
        --output "$decrypted" \
        --password-file "$keyfile" 2>/dev/null; then
        fail "decrypt command failed"
        return
    fi

    if [[ ! -f "$decrypted" ]]; then
        fail "decrypted file not created"
        return
    fi

    # Compare SHA-256
    local sha_after
    sha_after="$(sha256sum "$decrypted" | awk '{print $1}')"

    if [[ "$sha_before" == "$sha_after" ]]; then
        pass "SHA-256 matches after encrypt/decrypt roundtrip"
    else
        fail "SHA-256 mismatch" "before=$sha_before after=$sha_after"
    fi
}

test_dry_run_no_files() {
    run_test "--dry-run produces no output files"

    local input="${TMPDIR}/dryrun_input.bin"
    local encrypted="${TMPDIR}/dryrun_output.gpg"
    local keyfile="${TMPDIR}/dryrun_session.key"

    dd if=/dev/urandom of="$input" bs=512 count=1 2>/dev/null

    # Run encrypt with --dry-run
    bash "$SECURE_DIST" \
        --action encrypt \
        --input "$input" \
        --output "$encrypted" \
        --password-file "$keyfile" \
        --dry-run 2>/dev/null || true

    if [[ -f "$encrypted" ]]; then
        fail "dry-run created encrypted file (should not)"
        return
    fi

    if [[ -f "$keyfile" ]]; then
        fail "dry-run created session key file (should not)"
        return
    fi

    pass "--dry-run did not produce output files"
}

test_verify_passes() {
    run_test "verify operation passes after encrypt"

    local input="${TMPDIR}/verify_input.bin"
    local encrypted="${TMPDIR}/verify_input.bin.gpg"
    local keyfile="${TMPDIR}/verify_session.key"

    dd if=/dev/urandom of="$input" bs=1024 count=1 2>/dev/null

    # Encrypt first (generates .sha256 files)
    bash "$SECURE_DIST" \
        --action encrypt \
        --input "$input" \
        --output "$encrypted" \
        --password-file "$keyfile" 2>/dev/null

    # Verify
    if bash "$SECURE_DIST" \
        --action verify \
        --input "$input" \
        --output "$encrypted" \
        --password-file "$keyfile" 2>/dev/null; then
        pass "verify passed after encrypt"
    else
        fail "verify failed after encrypt"
    fi
}

test_error_file_not_found() {
    run_test "error: non-existent input file"

    if bash "$SECURE_DIST" \
        --action encrypt \
        --input "${TMPDIR}/nonexistent_file.bin" \
        --output "${TMPDIR}/out.gpg" \
        --password-file "${TMPDIR}/key.txt" 2>/dev/null; then
        fail "should have exited with error for missing input file"
    else
        pass "correctly failed for non-existent input file"
    fi
}

test_error_missing_params() {
    run_test "error: missing required --action parameter"

    if bash "$SECURE_DIST" \
        --input "${TMPDIR}/somefile.bin" 2>/dev/null; then
        fail "should have exited with error for missing --action"
    else
        pass "correctly failed for missing --action parameter"
    fi
}

test_decrypt_guide_generation() {
    run_test "generate-decrypt-guide.sh produces non-empty output"

    local guide_output="${TMPDIR}/README-DECRYPT.txt"

    if ! bash "$DECRYPT_GUIDE" \
        --filename "release-v1.0.0.zip" \
        --sha256 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" \
        --sha256-encrypted "abc123def456789012345678901234567890123456789012345678901234abcd" \
        --algo "AES-256" \
        --version "v1.0.0" \
        --output "$guide_output" 2>/dev/null; then
        fail "generate-decrypt-guide.sh exited with error"
        return
    fi

    if [[ ! -f "$guide_output" ]]; then
        fail "guide output file not created"
        return
    fi

    local size
    size="$(stat -c%s "$guide_output" 2>/dev/null || stat -f%z "$guide_output" 2>/dev/null)"
    if [[ "$size" -gt 0 ]]; then
        pass "decrypt guide generated (${size} bytes)"
    else
        fail "decrypt guide file is empty"
    fi
}

# --- Main ---

main() {
    echo "=========================================="
    echo " SecureRelease Distribution Tests"
    echo "=========================================="
    echo ""

    # Preflight checks
    if [[ ! -x "$SECURE_DIST" ]] && [[ ! -f "$SECURE_DIST" ]]; then
        echo "ERROR: secure-distribute.sh not found at $SECURE_DIST"
        exit 1
    fi
    if ! command -v gpg &>/dev/null; then
        echo "ERROR: gpg is required but not found"
        exit 1
    fi

    setup

    test_encrypt_decrypt_sha256
    test_dry_run_no_files
    test_verify_passes
    test_error_file_not_found
    test_error_missing_params
    test_decrypt_guide_generation

    echo ""
    echo "=========================================="
    echo " Results: $PASSED passed, $FAILED failed"
    echo "=========================================="

    if [[ "$FAILED" -gt 0 ]]; then
        exit 1
    fi
}

main "$@"
