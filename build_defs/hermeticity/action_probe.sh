#!/bin/bash
set -euo pipefail

mode=$1
out=$2

case "$mode" in
  file)
    # Deliberately absent from the rule's attrs and therefore its action key.
    cat "$PWD/build_defs/hermeticity/undeclared.txt" >"$out/value"
    ;;
  environment)
    # Deliberately inherited from buckd rather than passed as action env.
    printf '%s\n' "${KIBOU_AUDIT_SECRET-unset}" >"$out/value"
    ;;
  write)
    # Deliberately outside the declared output. The pytest driver points this at
    # its disposable runtime directory outside the watched Buck cell.
    : "${KIBOU_HERMETICITY_RUNTIME:?missing audit runtime}"
    mkdir -p "$KIBOU_HERMETICITY_RUNTIME"
    printf 'escaped\n' >"$KIBOU_HERMETICITY_RUNTIME/escape"
    printf 'wrote escape\n' >"$out/value"
    ;;
  *)
    printf 'unknown audit mode: %s\n' "$mode" >&2
    exit 2
    ;;
esac
