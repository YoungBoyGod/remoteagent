#!/usr/bin/env bash
set -euo pipefail

# secure-distribute.sh - GPG-based file encryption distribution tool
# Usage: ./secure-distribute.sh --action encrypt|decrypt|verify [OPTIONS]

readonly SCRIPT_NAME="$(basename "$0")"
readonly MIN_GPG_VERSION="2.2"
TEMP_FILES=()
JSON_OUTPUT=false
DRY_RUN=false
LOG_LEVEL="info"

# --- Logging ---

log() {
    local level="$1"; shift
    local ts
    ts="$(date '+%Y-%m-%d %H:%M:%S')"
    if [[ "$JSON_OUTPUT" == false ]]; then
        echo "[$ts] [$level] $*" >&2
    fi
}

log_info()  { log "INFO"  "$@"; }
log_warn()  { log "WARN"  "$@"; }
log_error() { log "ERROR" "$@"; }
log_debug() { [[ "$LOG_LEVEL" == "debug" ]] && log "DEBUG" "$@" || true; }

# --- Cleanup ---

cleanup() {
    for f in "${TEMP_FILES[@]}"; do
        if [[ -f "$f" ]]; then
            rm -f "$f"
            log_debug "Cleaned up temp file: $f"
        fi
    done
}
trap cleanup EXIT INT TERM

# --- JSON output helper ---

json_result() {
    # Accepts key=value pairs, outputs JSON object
    local out="{"
    local first=true
    for kv in "$@"; do
        local key="${kv%%=*}"
        local val="${kv#*=}"
        if [[ "$first" == true ]]; then
            first=false
        else
            out+=","
        fi
        out+="\"${key}\":\"${val}\""
    done
    out+="}"
    echo "$out"
}

# --- Help ---

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [OPTIONS]

GPG-based file encryption distribution tool.

Options:
  --action ACTION        Action to perform: encrypt, decrypt, verify
  --input FILE           Input file path
  --output FILE          Output file path
  --recipient EMAIL      Recipient email (for metadata, not asymmetric encryption)
  --signer-key EMAIL     Signer key identifier
  --password-file FILE   Path to session key file (generated on encrypt, read on decrypt/verify)
  --dry-run              Preview operations without executing
  --json                 Output results in JSON format
  --verbose              Enable debug logging
  --help                 Show this help message

Examples:
  # Encrypt a file
  $SCRIPT_NAME --action encrypt --input data.tar.gz --output data.tar.gz.gpg --password-file session.key

  # Decrypt a file
  $SCRIPT_NAME --action decrypt --input data.tar.gz.gpg --output data.tar.gz --password-file session.key

  # Verify file integrity
  $SCRIPT_NAME --action verify --input data.tar.gz --output data.tar.gz.gpg --password-file session.key
EOF
    exit 0
}

# --- Validation helpers ---

require_command() {
    if ! command -v "$1" &>/dev/null; then
        log_error "Required command not found: $1"
        exit 1
    fi
}

check_gpg_version() {
    require_command gpg
    local ver
    ver="$(gpg --version 2>/dev/null | head -n1 | grep -oP '\d+\.\d+(\.\d+)?')"
    if [[ -z "$ver" ]]; then
        log_error "Cannot determine GPG version"
        exit 1
    fi
    # Compare major.minor
    local major minor
    major="$(echo "$ver" | cut -d. -f1)"
    minor="$(echo "$ver" | cut -d. -f2)"
    local req_major req_minor
    req_major="$(echo "$MIN_GPG_VERSION" | cut -d. -f1)"
    req_minor="$(echo "$MIN_GPG_VERSION" | cut -d. -f2)"

    if (( major < req_major )) || { (( major == req_major )) && (( minor < req_minor )); }; then
        log_error "GPG version $ver is below minimum required $MIN_GPG_VERSION"
        exit 1
    fi
    log_info "GPG version $ver detected (>= $MIN_GPG_VERSION)"
}

check_file_exists() {
    local path="$1" label="$2"
    if [[ ! -f "$path" ]]; then
        log_error "$label file does not exist: $path"
        exit 1
    fi
}

check_file_readable() {
    local path="$1" label="$2"
    if [[ ! -r "$path" ]]; then
        log_error "$label file is not readable: $path"
        exit 1
    fi
}

check_dir_writable() {
    local path="$1" label="$2"
    local dir
    dir="$(dirname "$path")"
    if [[ ! -d "$dir" ]]; then
        log_error "Directory for $label does not exist: $dir"
        exit 1
    fi
    if [[ ! -w "$dir" ]]; then
        log_error "Directory for $label is not writable: $dir"
        exit 1
    fi
}

# --- SHA-256 helper (streaming, no full file load) ---

compute_sha256() {
    local file="$1"
    sha256sum "$file" | awk '{print $1}'
}

# --- Session key generation ---

generate_session_key() {
    local keyfile="$1"
    log_info "Generating 32-byte session key"
    dd if=/dev/urandom bs=32 count=1 2>/dev/null | base64 > "$keyfile"
    chmod 600 "$keyfile"
    log_info "Session key written to $keyfile (mode 600)"
}

# --- Actions ---

do_encrypt() {
    local input="$1" output="$2" password_file="$3"

    check_file_exists "$input" "Input"
    check_file_readable "$input" "Input"
    check_dir_writable "$output" "Output"
    check_dir_writable "$password_file" "Password"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY-RUN] Would generate session key at: $password_file"
        log_info "[DRY-RUN] Would encrypt: $input -> $output (AES256, ZLIB)"
        log_info "[DRY-RUN] Would compute SHA-256 for: $input -> ${input}.sha256"
        log_info "[DRY-RUN] Would compute SHA-256 for: $output -> ${output}.sha256"
        if [[ "$JSON_OUTPUT" == true ]]; then
            json_result \
                "status=dry-run" \
                "action=encrypt" \
                "input=$input" \
                "output=$output" \
                "session_key_file=$password_file"
        fi
        return 0
    fi

    # Generate session key
    generate_session_key "$password_file"

    # Encrypt with GPG symmetric AES256
    log_info "Encrypting $input -> $output"
    gpg --batch --yes \
        --passphrase-file "$password_file" \
        --symmetric \
        --cipher-algo AES256 \
        --compress-algo ZLIB \
        --output "$output" \
        "$input"
    log_info "Encryption complete"

    # Compute SHA-256 checksums (streaming via sha256sum)
    local sha_orig sha_enc
    sha_orig="$(compute_sha256 "$input")"
    echo "$sha_orig  $(basename "$input")" > "${input}.sha256"
    log_info "Original SHA-256: $sha_orig -> ${input}.sha256"

    sha_enc="$(compute_sha256 "$output")"
    echo "$sha_enc  $(basename "$output")" > "${output}.sha256"
    log_info "Encrypted SHA-256: $sha_enc -> ${output}.sha256"

    # JSON output
    if [[ "$JSON_OUTPUT" == true ]]; then
        json_result \
            "status=success" \
            "action=encrypt" \
            "encrypted_file=$output" \
            "sha256_original=$sha_orig" \
            "sha256_encrypted=$sha_enc" \
            "session_key_file=$password_file"
    fi
}

do_decrypt() {
    local input="$1" output="$2" password_file="$3"

    check_file_exists "$input" "Encrypted input"
    check_file_readable "$input" "Encrypted input"
    check_file_exists "$password_file" "Password"
    check_file_readable "$password_file" "Password"
    check_dir_writable "$output" "Output"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY-RUN] Would decrypt: $input -> $output using key $password_file"
        if [[ "$JSON_OUTPUT" == true ]]; then
            json_result \
                "status=dry-run" \
                "action=decrypt" \
                "input=$input" \
                "output=$output"
        fi
        return 0
    fi

    log_info "Decrypting $input -> $output"
    gpg --batch --yes \
        --passphrase-file "$password_file" \
        --decrypt \
        --output "$output" \
        "$input"
    log_info "Decryption complete"

    local sha_dec
    sha_dec="$(compute_sha256 "$output")"
    log_info "Decrypted file SHA-256: $sha_dec"

    if [[ "$JSON_OUTPUT" == true ]]; then
        json_result \
            "status=success" \
            "action=decrypt" \
            "decrypted_file=$output" \
            "sha256_decrypted=$sha_dec"
    fi
}

do_verify() {
    local input="$1" output="$2" password_file="$3"

    check_file_exists "$input" "Original"
    check_file_exists "$output" "Encrypted"

    local sha_input_file="${input}.sha256"
    local sha_output_file="${output}.sha256"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY-RUN] Would verify SHA-256 of: $input against $sha_input_file"
        log_info "[DRY-RUN] Would verify SHA-256 of: $output against $sha_output_file"
        if [[ "$JSON_OUTPUT" == true ]]; then
            json_result \
                "status=dry-run" \
                "action=verify" \
                "input=$input" \
                "output=$output"
        fi
        return 0
    fi

    local errors=0

    # Verify original file checksum
    if [[ -f "$sha_input_file" ]]; then
        local expected_orig actual_orig
        expected_orig="$(awk '{print $1}' "$sha_input_file")"
        actual_orig="$(compute_sha256 "$input")"
        if [[ "$expected_orig" == "$actual_orig" ]]; then
            log_info "Original file checksum OK: $actual_orig"
        else
            log_error "Original file checksum MISMATCH: expected=$expected_orig actual=$actual_orig"
            errors=$((errors + 1))
        fi
    else
        log_warn "No checksum file found for original: $sha_input_file"
        errors=$((errors + 1))
    fi

    # Verify encrypted file checksum
    if [[ -f "$sha_output_file" ]]; then
        local expected_enc actual_enc
        expected_enc="$(awk '{print $1}' "$sha_output_file")"
        actual_enc="$(compute_sha256 "$output")"
        if [[ "$expected_enc" == "$actual_enc" ]]; then
            log_info "Encrypted file checksum OK: $actual_enc"
        else
            log_error "Encrypted file checksum MISMATCH: expected=$expected_enc actual=$actual_enc"
            errors=$((errors + 1))
        fi
    else
        log_warn "No checksum file found for encrypted: $sha_output_file"
        errors=$((errors + 1))
    fi

    if (( errors > 0 )); then
        log_error "Verification failed with $errors error(s)"
        if [[ "$JSON_OUTPUT" == true ]]; then
            json_result "status=failed" "action=verify" "errors=$errors"
        fi
        exit 1
    fi

    log_info "Verification passed"
    if [[ "$JSON_OUTPUT" == true ]]; then
        json_result \
            "status=success" \
            "action=verify" \
            "sha256_original=${actual_orig:-unknown}" \
            "sha256_encrypted=${actual_enc:-unknown}"
    fi
}

# --- Main ---

main() {
    local action="" input="" output="" recipient="" signer_key="" password_file=""

    if [[ $# -eq 0 ]]; then
        usage
    fi

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --action)       action="$2";        shift 2 ;;
            --input)        input="$2";         shift 2 ;;
            --output)       output="$2";        shift 2 ;;
            --recipient)    recipient="$2";     shift 2 ;;
            --signer-key)   signer_key="$2";    shift 2 ;;
            --password-file) password_file="$2"; shift 2 ;;
            --dry-run)      DRY_RUN=true;       shift ;;
            --json)         JSON_OUTPUT=true;    shift ;;
            --verbose)      LOG_LEVEL="debug";   shift ;;
            --help|-h)      usage ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    # Validate required params
    if [[ -z "$action" ]]; then
        log_error "Missing required option: --action"
        exit 1
    fi
    if [[ -z "$input" ]]; then
        log_error "Missing required option: --input"
        exit 1
    fi

    # Check GPG
    check_gpg_version

    log_info "Action: $action | Input: $input | Output: ${output:-<auto>}"
    [[ -n "$recipient" ]] && log_info "Recipient: $recipient"
    [[ -n "$signer_key" ]] && log_info "Signer key: $signer_key"
    [[ "$DRY_RUN" == true ]] && log_info "Mode: DRY-RUN"

    case "$action" in
        encrypt)
            if [[ -z "$output" ]]; then
                output="${input}.gpg"
            fi
            if [[ -z "$password_file" ]]; then
                password_file="${input}.session.key"
            fi
            do_encrypt "$input" "$output" "$password_file"
            ;;
        decrypt)
            if [[ -z "$output" ]]; then
                output="${input%.gpg}"
                if [[ "$output" == "$input" ]]; then
                    output="${input}.decrypted"
                fi
            fi
            if [[ -z "$password_file" ]]; then
                log_error "Missing required option for decrypt: --password-file"
                exit 1
            fi
            do_decrypt "$input" "$output" "$password_file"
            ;;
        verify)
            if [[ -z "$output" ]]; then
                log_error "Missing required option for verify: --output (encrypted file)"
                exit 1
            fi
            do_verify "$input" "$output" "$password_file"
            ;;
        *)
            log_error "Unknown action: $action (expected: encrypt, decrypt, verify)"
            exit 1
            ;;
    esac
}

main "$@"
