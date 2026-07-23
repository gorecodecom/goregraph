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
{"type":"item.completed","item":{"id":"wrapped-sed","type":"command_execution","command":"/bin/zsh -lc 'sed -n \"1,20p\" /src/Wrapped.java'","exit_code":0}}
{"type":"item.completed","item":{"id":"option-pattern","type":"command_execution","command":"rg -e 'Model.ts' -g '*.ts' Worker /src/Handler.java","exit_code":0}}
{"type":"item.completed","item":{"id":"expression-target","type":"command_execution","command":"rg -e 'Model.ts' /src/Only.java","exit_code":0}}
{"type":"item.completed","item":{"id":"cat-numbered","type":"command_execution","command":"cat -n /src/Cat.java","exit_code":0}}
{"type":"item.completed","item":{"id":"compound-sed","type":"command_execution","command":"/bin/zsh -lc 'cd /work/service && sed -n \"1,20p\" src/Service.java'","exit_code":0}}
{"type":"item.completed","item":{"id":"attached-grep","type":"command_execution","command":"grep -eService /src/Attached.java","exit_code":0}}
{"type":"item.completed","item":{"id":"attached-rg","type":"command_execution","command":"rg -eWorker /src/AttachedRg.go","exit_code":0}}
{"type":"item.completed","item":{"id":"attached-sed-expression","type":"command_execution","command":"sed -e1,20p /src/AttachedSed.java","exit_code":0}}
{"type":"item.completed","item":{"id":"attached-sed-file","type":"command_execution","command":"sed -f/src/Script.java /src/FileProgram.java","exit_code":0}}
{"type":"item.completed","item":{"id":"compound-pipeline","type":"command_execution","command":"/bin/zsh -lc 'grep -n Service src/Compound.java | sed -n \"1,20p\" src/Compound.java'","exit_code":0}}
{"type":"item.completed","item":{"id":"direct-read","type":"file_change","path":"/src/worker.py","status":"failed"}}
{"type":"item.completed","item":{"id":"cli-full","type":"command_execution","command":"goregraph context /work --query route","aggregated_output":"# Context Pack\n\nContext ID: full-two\n"}}
{"type":"item.completed","item":{"id":"mcp-full","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"{\"context_id\":\"full-one\"}"}]}}}
{"type":"item.completed","item":{"id":"mcp-duplicate","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"# Context Pack\n\nContext ID: compact-one\nDuplicate of: full-one\n"}]}}}
{"type":"item.completed","item":{"id":"cli-repeat","type":"command_execution","command":"goregraph context /work --query retry","aggregated_output":"# Context Pack\n\nContext ID: full-two\n"}}
{"type":"item.completed","item":{"id":"web-search","type":"web_search","query":"route"}}
{"type":"item.completed","item":{"id":"collaboration","type":"collab_tool_call","target":"helper"}}
{"type":"item.completed","item":{"id":"assistant-message","type":"agent_message","text":"not a tool"}}
{"type":"turn.completed","usage":{"input_tokens":60000,"cached_input_tokens":10000,"output_tokens":30000,"total_tokens":100000}}
EOF

expected_header=$'tool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tincluded_source_rereads\tunique_source_files'
header=$(bash "$analyzer" --header "$temporary_directory/transcript.jsonl")
[ "$header" = "$expected_header" ] || fail "header = $header"

row=$(bash "$analyzer" "$temporary_directory/transcript.jsonl")
[ "$row" = $'21\t4\t2\t1\t1\t13\t8\t0\t13' ] || fail "row = $row"

cat >"$temporary_directory/included-rereads.jsonl" <<'EOF'
{"type":"item.completed","item":{"id":"before-pack","type":"command_execution","command":"cat /work/services/catalog/src/CatalogService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"json-pack","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"{\"context_id\":\"json-complete\",\"source_coverage\":\"complete\",\"source_sections\":[{\"project\":\"services/catalog\",\"path\":\"src/CatalogService.java\"}]}"}]}}}
{"type":"item.completed","item":{"id":"json-reread","type":"command_execution","command":"sed -n '1,20p' /work/services/catalog/src/CatalogService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"json-reread","type":"command_execution","command":"sed -n '1,20p' /work/services/catalog/src/CatalogService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"markdown-pack","type":"command_execution","command":"goregraph context /work --query jobs","aggregated_output":"# GoreGraph Context\n\nContext ID: markdown-complete\nSource coverage: complete\n\n## Source sections\n\n### 1. `services/jobs/src/JobService.java:10-20`\n"}}
{"type":"item.completed","item":{"id":"markdown-reread","type":"command_execution","command":"rg -n delete /work/services/jobs/src/JobService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"partial-pack","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"{\"context_id\":\"json-partial\",\"source_coverage\":\"partial\",\"source_sections\":[{\"project\":\"services/worker\",\"path\":\"src/Worker.go\"}],\"source_omissions\":[{\"project\":\"services/worker\",\"path\":\"src/Missing.go\"}]}"}]}}}
{"type":"item.completed","item":{"id":"partial-read","type":"command_execution","command":"cat /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"turn.completed","usage":{"total_tokens":100}}
EOF

reread_row=$(bash "$analyzer" "$temporary_directory/included-rereads.jsonl")
IFS=$'\t' read -r _ _ _ _ _ _ _ included_rereads _ extra <<EOF
$reread_row
EOF
[ -z "${extra:-}" ] || fail "included reread row has extra fields: $reread_row"
[ "$included_rereads" = "2" ] || fail "included source rereads = $included_rereads, row = $reread_row"

cat >"$temporary_directory/fallback-usage.jsonl" <<'EOF'
{"type":"item.completed","item":{"id":"search","type":"web_search","query":"route"}}
{"type":"turn.completed","usage":{"input_tokens":12,"output_tokens":3}}
EOF
tokens=$(bash "$analyzer" --tokens "$temporary_directory/fallback-usage.jsonl")
[ "$tokens" = "15" ] || fail "fallback tokens = $tokens"

if bash "$analyzer" "$temporary_directory/missing.jsonl" >/dev/null 2>&1; then
  fail "missing transcript passed"
fi

printf '{"type":"item.completed","item":{"id":"message","type":"agent_message"}}\n' >"$temporary_directory/unparseable.jsonl"
if bash "$analyzer" "$temporary_directory/unparseable.jsonl" >/dev/null 2>&1; then
  fail "unparseable transcript passed"
fi

printf '{"type":"item.completed","item":{"id":"broken","type":"command_execution","command":"cat /src/Broken.java"}},\n' >"$temporary_directory/malformed.jsonl"
if bash "$analyzer" "$temporary_directory/malformed.jsonl" >/dev/null 2>&1; then
  fail "malformed JSONL passed"
fi

printf '{"type":"item.completed","item":{"id":"unknown","type":"future_tool"}}\n' >"$temporary_directory/unknown-item.jsonl"
if bash "$analyzer" "$temporary_directory/unknown-item.jsonl" >/dev/null 2>&1; then
  fail "unknown completed item passed"
fi

cat >"$temporary_directory/conflicting-id.jsonl" <<'EOF'
{"type":"item.completed","item":{"id":"same","type":"web_search","query":"first"}}
{"type":"item.completed","item":{"id":"same","type":"web_search","query":"second"}}
EOF
if bash "$analyzer" "$temporary_directory/conflicting-id.jsonl" >/dev/null 2>&1; then
  fail "conflicting terminal item ID passed"
fi

printf 'PASS: analyze-agent-context-log\n'
