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

cat >"$temporary_directory/transcript.jsonl" <<'EOF'
{"type":"thread.started","thread_id":"thread-1"}
{"type":"item.started","item":{"id":"command-pattern","type":"command_execution","command":"rg -n 'Service.java' /src"}}
{"type":"item.completed","item":{"id":"command-pattern","type":"command_execution","command":"rg -n 'Service.java' /src","exit_code":0}}
{"type":"item.completed","item":{"id":"command-pattern","type":"command_execution","command":"rg -n 'Service.java' /src","exit_code":0}}
{"type":"item.completed","item":{"id":"find-pattern","type":"command_execution","command":"find . -name '*.java'","exit_code":1}}
{"type":"item.completed","item":{"id":"source-read","type":"command_execution","command":"sed -n '1,20p' /src/Service.java","exit_code":0}}
{"type":"item.completed","item":{"id":"wrapped-search","type":"command_execution","command":"/bin/zsh -lc 'grep -n Worker /src/Worker.go'","exit_code":0}}
{"type":"item.completed","item":{"id":"option-pattern","type":"command_execution","command":"rg -e 'Model.ts' -g '*.ts' Worker /src/Handler.java","exit_code":0}}
{"type":"item.completed","item":{"id":"expression-target","type":"command_execution","command":"rg -e 'Model.ts' /src/Only.java","exit_code":0}}
{"type":"item.completed","item":{"id":"cat-numbered","type":"command_execution","command":"cat -n /src/Cat.java","exit_code":0}}
{"type":"item.completed","item":{"id":"direct-read","type":"file_read","path":"/src/worker.py","status":"failed"}}
{"type":"item.completed","item":{"id":"cli-full","type":"command_execution","command":"goregraph context /work --query route","aggregated_output":"# Context Pack\ncontext_id: full-two\n"}}
{"type":"item.completed","item":{"id":"mcp-full","type":"mcp_tool_call","tool":"task_context","result":"{\"context_id\":\"full-one\"}"}}
{"type":"item.completed","item":{"id":"mcp-duplicate","type":"mcp_tool_call","tool":"task_context","result":"{\"context_id\":\"compact-one\",\"duplicate_of\":\"full-one\"}"}}
{"type":"item.completed","item":{"id":"cli-repeat","type":"command_execution","command":"goregraph context /work --query retry","aggregated_output":"# Context Pack\ncontext_id: full-two\n"}}
{"type":"item.completed","item":{"id":"web-search","type":"web_search","query":"route"}}
{"type":"item.completed","item":{"id":"assistant-message","type":"agent_message","text":"not a tool"}}
{"type":"turn.completed","usage":{"input_tokens":60000,"cached_input_tokens":10000,"output_tokens":30000,"total_tokens":100000}}
EOF

expected_header=$'tool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tunique_source_files'
header=$(bash "$analyzer" --header "$temporary_directory/transcript.jsonl")
[ "$header" = "$expected_header" ] || fail "header = $header"

row=$(bash "$analyzer" "$temporary_directory/transcript.jsonl")
[ "$row" = $'13\t4\t2\t1\t1\t6\t3\t6' ] || fail "row = $row"

if bash "$analyzer" "$temporary_directory/missing.jsonl" >/dev/null 2>&1; then
  fail "missing transcript passed"
fi

printf '{"type":"item.completed","item":{"id":"message","type":"agent_message"}}\n' >"$temporary_directory/unparseable.jsonl"
if bash "$analyzer" "$temporary_directory/unparseable.jsonl" >/dev/null 2>&1; then
  fail "unparseable transcript passed"
fi

printf '{"type":"item.completed","item":{"id":"broken","type":"command_execution","command":"cat /src/Broken.java"}\n' >"$temporary_directory/malformed.jsonl"
if bash "$analyzer" "$temporary_directory/malformed.jsonl" >/dev/null 2>&1; then
  fail "malformed JSONL passed"
fi

printf 'PASS: analyze-agent-context-log\n'
