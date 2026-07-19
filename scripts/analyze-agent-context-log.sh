#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
helper="$script_dir/analyze-agent-context-log.go"

[ -f "$helper" ] && [ -r "$helper" ] || {
  printf 'error: analyzer helper is not a readable regular file: %s\n' "$helper" >&2
  exit 2
}
command -v go >/dev/null 2>&1 || {
  printf 'error: required command not found: go\n' >&2
  exit 2
}

exec go run "$helper" "$@"
