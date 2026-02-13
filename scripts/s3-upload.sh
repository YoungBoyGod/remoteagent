#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# s3-upload.sh - S3 upload with presigned URL generation and SCP fallback
# ============================================================================

readonly SCRIPT_NAME="$(basename "$0")"
readonly MAX_RETRIES=3
readonly RETRY_BASE_DELAY=2

# Defaults
FILE=""
BUCKET="secure-releases"
PREFIX="encrypted/"
EXPIRES=86400
FALLBACK=""
SCP_TARGET=""
SCP_URL_BASE=""
OUTPUT_FORMAT="json"
DRY_RUN=false
TEMP_FILES=()

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
log() {
    echo "[$(date '+%Y-%m-%dT%H:%M:%S%z')] [$1] $2" >&2
}

log_info()  { log "INFO"  "$1"; }
log_warn()  { log "WARN"  "$1"; }
log_error() { log "ERROR" "$1"; }

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
cleanup() {
    for f in "${TEMP_FILES[@]}"; do
        rm -f "$f" 2>/dev/null || true
    done
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------
usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [OPTIONS]

Upload a file to S3 and generate a presigned download URL.
Falls back to SCP when configured or when AWS CLI is unavailable.

Options:
  --file PATH           File to upload (required)
  --bucket NAME         S3 bucket name (default: secure-releases)
  --prefix PREFIX       S3 key prefix (default: encrypted/)
  --expires SECONDS     Presigned URL expiry in seconds (default: 86400)
  --fallback MODE       Fallback mode: scp (optional)
  --scp-target DEST     SCP destination, e.g. user@host:/path/
  --scp-url-base URL    Base URL for SCP downloads
  --output FORMAT       Output format: json (default: json)
  --dry-run             Show what would be done without executing
  --help                Show this help message

Examples:
  $SCRIPT_NAME --file release.gpg --bucket secure-releases --prefix encrypted/
  $SCRIPT_NAME --file release.gpg --fallback scp --scp-target user@host:/data/ --scp-url-base https://dl.example.com/
EOF
    exit 0
}

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
parse_args() {
    if [[ $# -eq 0 ]]; then
        usage
    fi

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --file)        FILE="$2";         shift 2 ;;
            --bucket)      BUCKET="$2";       shift 2 ;;
            --prefix)      PREFIX="$2";       shift 2 ;;
            --expires)     EXPIRES="$2";      shift 2 ;;
            --fallback)    FALLBACK="$2";     shift 2 ;;
            --scp-target)  SCP_TARGET="$2";   shift 2 ;;
            --scp-url-base) SCP_URL_BASE="$2"; shift 2 ;;
            --output)      OUTPUT_FORMAT="$2"; shift 2 ;;
            --dry-run)     DRY_RUN=true;      shift ;;
            --help)        usage ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    if [[ -z "$FILE" ]]; then
        log_error "--file is required"
        exit 1
    fi

    if [[ ! -f "$FILE" ]]; then
        log_error "File not found: $FILE"
        exit 1
    fi
}

# ---------------------------------------------------------------------------
# Retry with exponential backoff
# ---------------------------------------------------------------------------
retry() {
    local attempt=1
    local cmd=("$@")

    while (( attempt <= MAX_RETRIES )); do
        if "${cmd[@]}"; then
            return 0
        fi
        local delay=$(( RETRY_BASE_DELAY ** attempt ))
        log_warn "Attempt $attempt/$MAX_RETRIES failed. Retrying in ${delay}s..."
        sleep "$delay"
        (( attempt++ ))
    done

    log_error "All $MAX_RETRIES attempts failed for: ${cmd[*]}"
    return 1
}

# ---------------------------------------------------------------------------
# File size (human readable)
# ---------------------------------------------------------------------------
get_file_size() {
    local file="$1"
    local bytes
    bytes=$(stat -c%s "$file" 2>/dev/null || stat -f%z "$file" 2>/dev/null || echo 0)

    if (( bytes >= 1073741824 )); then
        printf "%.2fGB" "$(echo "scale=2; $bytes / 1073741824" | bc)"
    elif (( bytes >= 1048576 )); then
        printf "%.2fMB" "$(echo "scale=2; $bytes / 1048576" | bc)"
    elif (( bytes >= 1024 )); then
        printf "%.2fKB" "$(echo "scale=2; $bytes / 1024" | bc)"
    else
        printf "%dB" "$bytes"
    fi
}

# ---------------------------------------------------------------------------
# Compute expiry timestamp
# ---------------------------------------------------------------------------
get_expires_at() {
    date -u -d "+${EXPIRES} seconds" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null \
        || date -u -v "+${EXPIRES}S" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null \
        || echo "unknown"
}

# ---------------------------------------------------------------------------
# JSON output
# ---------------------------------------------------------------------------
emit_json() {
    local status="$1" url="$2" storage="$3" file_size="$4" expires_at="$5"
    cat <<EOJSON
{"status":"${status}","url":"${url}","expires_at":"${expires_at}","storage":"${storage}","file_size":"${file_size}"}
EOJSON
}

# ---------------------------------------------------------------------------
# S3 upload
# ---------------------------------------------------------------------------
do_s3_upload() {
    local filename
    filename="$(basename "$FILE")"
    local s3_key="s3://${BUCKET}/${PREFIX}${filename}"
    local file_size
    file_size="$(get_file_size "$FILE")"

    log_info "Uploading $FILE ($file_size) -> $s3_key"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY-RUN] aws s3 cp $FILE $s3_key"
        log_info "[DRY-RUN] aws s3 presign $s3_key --expires-in $EXPIRES"
        emit_json "dry-run" "https://${BUCKET}.s3.amazonaws.com/${PREFIX}${filename}" "s3" "$file_size" "$(get_expires_at)"
        return 0
    fi

    retry aws s3 cp "$FILE" "$s3_key"
    log_info "Upload complete"

    log_info "Generating presigned URL (expires in ${EXPIRES}s)..."
    local presigned_url
    presigned_url="$(aws s3 presign "$s3_key" --expires-in "$EXPIRES")"

    local expires_at
    expires_at="$(get_expires_at)"

    emit_json "success" "$presigned_url" "s3" "$file_size" "$expires_at"
}

# ---------------------------------------------------------------------------
# SCP upload
# ---------------------------------------------------------------------------
do_scp_upload() {
    local filename
    filename="$(basename "$FILE")"
    local file_size
    file_size="$(get_file_size "$FILE")"

    if [[ -z "$SCP_TARGET" ]]; then
        log_error "--scp-target is required for SCP mode"
        exit 1
    fi

    if [[ -z "$SCP_URL_BASE" ]]; then
        log_error "--scp-url-base is required for SCP mode"
        exit 1
    fi

    local dest="${SCP_TARGET}${filename}"
    log_info "Uploading via SCP: $FILE ($file_size) -> $dest"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY-RUN] scp $FILE $dest"
        emit_json "dry-run" "${SCP_URL_BASE}${filename}" "scp" "$file_size" "N/A"
        return 0
    fi

    retry scp "$FILE" "$dest"
    log_info "SCP upload complete"

    local download_url="${SCP_URL_BASE}${filename}"
    emit_json "success" "$download_url" "scp" "$file_size" "N/A"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    parse_args "$@"

    local use_scp=false

    if [[ "$FALLBACK" == "scp" ]]; then
        use_scp=true
    elif ! command -v aws &>/dev/null; then
        log_warn "AWS CLI not found, falling back to SCP"
        use_scp=true
    fi

    if [[ "$use_scp" == true ]]; then
        do_scp_upload
    else
        do_s3_upload
    fi
}

main "$@"
