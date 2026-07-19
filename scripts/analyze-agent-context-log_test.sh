#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
analyzer="$script_dir/analyze-agent-context-log.sh"
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-context-log-test.XXXXXX")
cleanup() {
  status=$?
  trap - EXIT
  rm -rf -- "$temporary_directory"
  exit "$status"
}
trap cleanup EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

cat >"$temporary_directory/transcript.log" <<'EOF'
{"type":"tool_call","tool_name":"exec","command":"goregraph context /work --query route"}
{"type":"tool_call","tool_name":"task_context","arguments":{"query":"route"}}
{"type":"mcp_call","name":"task_context","arguments":{"query":"route"}}
{"type":"tool_call","tool_name":"exec","command":"rg -n Service /work/src/Service.java"}
{"type":"tool_call","tool_name":"exec","command":"sed -n '1,80p' /work/src/Service.java"}
{"type":"tool_call","tool_name":"exec","command":"nl -ba /work/src/Service.java"}
{"type":"tool_call","tool_name":"read_file","path":"/work/src/worker.py"}
{"type":"tool_call","tool_name":"exec","command":"cat /work/README.md"}
{"type":"tool_call","tool_name":"exec","command":"make test"}
{"type":"exec","cmd":"head -n 20 /work/src/handler.ts"}
{"type":"task_context","context_id":"third-pack"}
{"context_id":"first-pack"}
{"context_id":"first-pack"}
{"context_id":"second-pack","duplicate_of":"first-pack"}
EOF

expected_header=$'tool_calls\tgoregraph_calls\tfull_context_packs\tduplicate_context_packs\traw_navigation_calls\tsource_read_calls\tunique_source_files'
header=$(bash "$analyzer" --header "$temporary_directory/transcript.log")
[ "$header" = "$expected_header" ] || fail "header = $header"

row=$(bash "$analyzer" "$temporary_directory/transcript.log")
[ "$row" = $'11\t4\t2\t2\t5\t4\t3' ] || fail "row = $row"

if bash "$analyzer" "$temporary_directory/missing.log" >/dev/null 2>&1; then
  fail "missing transcript passed"
fi

printf 'unrelated output only\n' >"$temporary_directory/unparseable.log"
if bash "$analyzer" "$temporary_directory/unparseable.log" >/dev/null 2>&1; then
  fail "unparseable transcript passed"
fi

printf 'PASS: analyze-agent-context-log\n'
