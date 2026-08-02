#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: CONFIRM_RESTORE=YES $0 /backups/repositories_<timestamp>.tar.gz" >&2
    exit 2
fi

if [[ "${CONFIRM_RESTORE:-}" != "YES" ]]; then
    echo "Restore is destructive. Re-run with CONFIRM_RESTORE=YES." >&2
    exit 2
fi

ARCHIVE="$(realpath -- "$1")"
TARGET="${REPOSITORY_BASE_PATH:-/data/luxview/repositories}"
TARGET_PARENT="$(dirname "$TARGET")"

if [[ ! -f "$ARCHIVE" ]]; then
    echo "Backup archive not found: $ARCHIVE" >&2
    exit 1
fi

if [[ "$(basename "$TARGET")" != "repositories" ]]; then
    echo "REPOSITORY_BASE_PATH must end in 'repositories' for safe restore." >&2
    exit 1
fi

ARCHIVE_ENTRIES="$(tar -tzf "$ARCHIVE")"
if [[ -z "$ARCHIVE_ENTRIES" ]] || printf '%s\n' "$ARCHIVE_ENTRIES" | grep -Eq '(^/|(^|/)\.\./|^repositories/\.\.)'; then
    echo "Backup archive contains unsafe paths." >&2
    exit 1
fi
if ! printf '%s\n' "$ARCHIVE_ENTRIES" | grep -qE '^repositories(/|$)'; then
    echo "Backup archive does not contain the hosted repositories directory." >&2
    exit 1
fi
mkdir -p "$TARGET_PARENT"
tar -xzf "$ARCHIVE" -C "$TARGET_PARENT"
chown -R "${LUXVIEW_OWNER:-luxview:luxview}" "$TARGET" 2>/dev/null || true
echo "Hosted repositories restored to $TARGET"
