#!/bin/bash
set -euo pipefail

expected=$1
runtime=$2
trace="$runtime/network-$expected.trace"
port=$(cat "$runtime/port")

set +e
/usr/bin/strace -f -qq -s 4096 -e trace=network -o "$trace" \
  /bin/bash -c '
    set -e
    exec 3<>"/dev/tcp/127.0.0.1/$1"
    printf "ping\n" >&3
    IFS= read -r response <&3
    test "$response" = alpha
  ' kibou-network-probe "$port"
status=$?
set -e

if [[ $status -eq 0 ]]; then
  actual=success
else
  actual=blocked
fi

if [[ "$actual" != "$expected" ]]; then
  printf 'expected network %s, got %s\n' "$expected" "$actual" >&2
  exit 1
fi
