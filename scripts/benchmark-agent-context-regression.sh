#!/usr/bin/env bash

set -euo pipefail
script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
repo_root=$(cd -P -- "$script_dir/.." && pwd -P)
exec go run "$repo_root/scripts/agent-context-regression" "$@"
