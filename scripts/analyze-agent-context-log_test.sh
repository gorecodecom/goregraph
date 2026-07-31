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
{"type":"turn.completed","usage":{"input_tokens":182151,"cached_input_tokens":146944,"output_tokens":6160,"reasoning_output_tokens":3195}}
EOF

expected_header=$'tool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tunique_source_files\texternal_skill_read_calls'
header=$(bash "$analyzer" --header "$temporary_directory/transcript.jsonl")
[ "$header" = "$expected_header" ] || fail "header = $header"

row=$(bash "$analyzer" "$temporary_directory/transcript.jsonl")
[ "$row" = $'21\t4\t2\t1\t1\t15\t8\t0\t15\t0\t13\t0' ] || fail "row = $row"

usage=$(bash "$analyzer" --usage "$temporary_directory/transcript.jsonl")
[ "$usage" = $'182151\t146944\t35207\t6160\t3195\t188311\t41367' ] ||
  fail "usage row = $usage"

cat >"$temporary_directory/invalid-usage.jsonl" <<'EOF'
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":11,"output_tokens":1,"reasoning_output_tokens":0}}
EOF
if bash "$analyzer" --usage "$temporary_directory/invalid-usage.jsonl" \
  >"$temporary_directory/invalid-usage.stdout" \
  2>"$temporary_directory/invalid-usage.stderr"; then
  fail "usage with cached input greater than input passed"
fi
grep -q 'cached_input_tokens exceeds input_tokens' \
  "$temporary_directory/invalid-usage.stderr" ||
  fail "invalid usage error was not specific"

cat >"$temporary_directory/included-rereads.jsonl" <<'EOF'
{"type":"item.completed","item":{"id":"before-pack","type":"command_execution","command":"cat /work/services/catalog/src/CatalogService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"json-pack","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"{\"context_id\":\"json-complete\",\"source_coverage\":\"complete\",\"source_sections\":[{\"project\":\"services/catalog\",\"path\":\"src/CatalogService.java\"}]}"}]}}}
{"type":"item.completed","item":{"id":"json-reread","type":"command_execution","command":"sed -n '1,20p' /work/services/catalog/src/CatalogService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"json-reread","type":"command_execution","command":"sed -n '1,20p' /work/services/catalog/src/CatalogService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"markdown-pack","type":"command_execution","command":"goregraph context /work --query jobs","aggregated_output":"# GoreGraph Context\n\nContext ID: markdown-complete\nSource coverage: complete\n\n## Source sections\n\n### 1. `services/jobs/src/JobService.java:10-20`\n"}}
{"type":"item.completed","item":{"id":"markdown-reread","type":"command_execution","command":"rg -n delete /work/services/jobs/src/JobService.java","exit_code":0}}
{"type":"item.completed","item":{"id":"partial-pack","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"{\"context_id\":\"json-partial\",\"source_coverage\":\"partial\",\"source_sections\":[{\"project\":\"services/worker\",\"path\":\"src/Worker.go\",\"start_line\":10,\"end_line\":20}],\"source_omissions\":[{\"project\":\"services/worker\",\"path\":\"src/Missing.go\"}]}"}]}}}
{"type":"item.completed","item":{"id":"partial-overlap","type":"command_execution","command":"sed -n '12,15p' /work/services/worker/src/Worker.go","exit_code":0}}
{"type":"item.completed","item":{"id":"partial-non-overlap","type":"command_execution","command":"sed -n '30,40p' /work/services/worker/src/Worker.go","exit_code":0}}
{"type":"item.completed","item":{"id":"partial-whole-read","type":"command_execution","command":"rg -n worker /work/services/worker/src/Worker.go","exit_code":0}}
{"type":"item.completed","item":{"id":"partial-read","type":"command_execution","command":"cat /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"turn.completed","usage":{"total_tokens":100}}
EOF

reread_row=$(bash "$analyzer" "$temporary_directory/included-rereads.jsonl")
[ "$reread_row" = $'10\t3\t3\t0\t0\t7\t5\t0\t7\t4\t4\t0' ] ||
  fail "included reread row = $reread_row"
IFS=$'\t' read -r _ _ _ _ _ _ _ bounded_reads unauthorized_reads included_rereads _ _ extra <<EOF
$reread_row
EOF
[ -z "${extra:-}" ] || fail "included reread row has extra fields: $reread_row"
[ "$bounded_reads" = "0" ] || fail "bounded omission reads = $bounded_reads, row = $reread_row"
[ "$unauthorized_reads" = "7" ] || fail "unauthorized source reads = $unauthorized_reads, row = $reread_row"
[ "$included_rereads" = "4" ] || fail "included source rereads = $included_rereads, row = $reread_row"

cat >"$temporary_directory/bounded-omissions.jsonl" <<'EOF'
{"type":"item.completed","item":{"id":"before-pack","type":"command_execution","command":"sed -n '30,32p' /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"item.completed","item":{"id":"json-pack","type":"mcp_tool_call","tool":"task_context","result":{"content":[{"type":"text","text":"{\"context_id\":\"json-partial\",\"source_coverage\":\"partial\",\"source_sections\":[{\"project\":\"services/worker\",\"path\":\"src/Worker.go\",\"start_line\":10,\"end_line\":20}],\"source_omissions\":[{\"project\":\"services/worker\",\"path\":\"src/Missing.go\",\"start_line\":30,\"end_line\":40},{\"project\":\"services/worker\",\"path\":\"src/Other.go\",\"start_line\":50,\"end_line\":60},{\"project\":\"services/worker\",\"path\":\"src/Unbounded.go\"}]}"}]}}}
{"type":"item.completed","item":{"id":"exact","type":"command_execution","command":"sed -n '30,40p' /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"item.completed","item":{"id":"exact","type":"command_execution","command":"sed -n '30,40p' /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"item.completed","item":{"id":"subset","type":"command_execution","command":"sed -n '32,35p' /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"item.completed","item":{"id":"widened","type":"command_execution","command":"sed -n '29,40p' /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"item.completed","item":{"id":"wrong-path","type":"command_execution","command":"sed -n '30,40p' /work/services/worker/src/Wrong.go","exit_code":0}}
{"type":"item.completed","item":{"id":"unbounded","type":"command_execution","command":"cat /work/services/worker/src/Unbounded.go","exit_code":0}}
{"type":"item.completed","item":{"id":"included-overlap","type":"command_execution","command":"sed -n '12,15p' /work/services/worker/src/Worker.go","exit_code":0}}
{"type":"item.completed","item":{"id":"search","type":"command_execution","command":"rg -n delete /work/services/worker/src/Missing.go","exit_code":0}}
{"type":"item.completed","item":{"id":"inventory","type":"command_execution","command":"find /work/services/worker -name '*.go'","exit_code":0}}
{"type":"item.completed","item":{"id":"compound","type":"command_execution","command":"/bin/zsh -lc 'sed -n \"30,32p\" /work/services/worker/src/Missing.go; sed -n \"30,32p\" /work/services/worker/src/Wrong.go'","exit_code":0}}
{"type":"item.completed","item":{"id":"markdown-pack","type":"command_execution","command":"goregraph context /work --query jobs","aggregated_output":"# GoreGraph Context\n\nContext ID: markdown-partial\nSource coverage: partial\n\n## Source omissions\n- `services/jobs/src/JobRepository.java:70-80` — role: persistence\n"}}
{"type":"item.completed","item":{"id":"markdown-exact","type":"command_execution","command":"sed -n '72,75p' /work/services/jobs/src/JobRepository.java","exit_code":0}}
{"type":"turn.completed","usage":{"total_tokens":100}}
EOF

bounded_row=$(bash "$analyzer" "$temporary_directory/bounded-omissions.jsonl")
[ "$bounded_row" = $'13\t2\t2\t0\t0\t11\t9\t3\t8\t1\t5\t0' ] ||
  fail "bounded omission row = $bounded_row"

legacy_tokens=$(bash "$analyzer" --tokens "$temporary_directory/included-rereads.jsonl")
[ "$legacy_tokens" = "100" ] || fail "legacy tokens = $legacy_tokens"

mkdir -p "$temporary_directory/workspace/testdata/skills/example"
cat >"$temporary_directory/skill-reads.jsonl" <<EOF
{"type":"item.completed","item":{"id":"skill-one","type":"command_execution","command":"cat /Users/me/.codex/skills/brainstorming/SKILL.md","exit_code":0}}
{"type":"item.completed","item":{"id":"ordinary-external","type":"command_execution","command":"cat /opt/source/config.json","exit_code":0}}
{"type":"item.completed","item":{"id":"workspace-skill","type":"command_execution","command":"cat $temporary_directory/workspace/testdata/skills/example/SKILL.md","exit_code":0}}
{"type":"item.completed","item":{"id":"skill-two","type":"command_execution","command":"rg -n Rule /opt/codex/plugins/vendor/skills/tdd/references/guide.md","exit_code":0}}
{"type":"item.completed","item":{"id":"skill-two-targets","type":"command_execution","command":"cat /opt/codex/plugins/vendor/skills/review/SKILL.md /opt/codex/plugins/vendor/skills/review/references/checklist.md","exit_code":0}}
{"type":"item.completed","item":{"id":"path-executable","type":"command_execution","command":"/bin/cat /opt/codex/plugins/vendor/skills/review/SKILL.md","exit_code":0}}
{"type":"item.completed","item":{"id":"patternless-inventory","type":"command_execution","command":"/usr/bin/rg --files /opt/codex/plugins/vendor/skills/review","exit_code":0}}
{"type":"item.completed","item":{"id":"relative-skill-directory","type":"command_execution","command":"/bin/zsh -lc 'cd ../external/skills/review && cat SKILL.md'","exit_code":0}}
{"type":"item.completed","item":{"id":"workspace-inventory","type":"command_execution","command":"/usr/bin/rg --files $temporary_directory/workspace/testdata/skills/example","exit_code":0}}
{"type":"item.completed","item":{"id":"workspace-relative","type":"command_execution","command":"/bin/zsh -lc 'cd testdata/skills/example && /bin/cat SKILL.md'","exit_code":0}}
{"type":"turn.completed","usage":{"input_tokens":20,"cached_input_tokens":5,"output_tokens":5,"reasoning_output_tokens":1}}
EOF

header=$(bash "$analyzer" --header "$temporary_directory/transcript.jsonl")
case "$header" in
  *$'\texternal_skill_read_calls') ;;
  *) fail "analyzer header lacks external_skill_read_calls: $header" ;;
esac

skill_row=$(bash "$analyzer" \
  --workspace "$temporary_directory/workspace" \
  "$temporary_directory/skill-reads.jsonl")
[ "${skill_row##*$'\t'}" = "6" ] || fail "skill-read count row = $skill_row"

skill_evidence=$(bash "$analyzer" \
  --workspace "$temporary_directory/workspace" \
  --skill-reads "$temporary_directory/skill-reads.jsonl")
printf '%s\n' "$skill_evidence" | grep -q '"event_order":1' ||
  fail "first skill event order missing"
printf '%s\n' "$skill_evidence" | grep -q '/Users/me/.codex/skills/brainstorming/SKILL.md' ||
  fail "first generic skill target missing"
printf '%s\n' "$skill_evidence" | grep -q '"event_order":4' ||
  fail "second skill event order missing"
printf '%s\n' "$skill_evidence" | grep -q '/opt/codex/plugins/vendor/skills/tdd/references/guide.md' ||
  fail "second generic skill target missing"
case "$skill_evidence" in
  *ordinary-external*|*workspace-skill*) fail "non-contaminating target entered evidence" ;;
esac
target_count=$(printf '%s\n' "$skill_evidence" |
  grep -o '"item_id":"skill-two-targets"' |
  wc -l | tr -d ' ')
[ "$target_count" = "2" ] || fail "multi-target skill evidence = $skill_evidence"
printf '%s\n' "$skill_evidence" | grep -q '"target":"/opt/codex/plugins/vendor/skills/review/SKILL.md"' ||
  fail "path-qualified executable target missing"
printf '%s\n' "$skill_evidence" | grep -q '"target":"/opt/codex/plugins/vendor/skills/review"' ||
  fail "patternless inventory target missing"
case "$skill_evidence" in
  *'"item_id":"relative-skill-directory"'*'/external/skills/review/SKILL.md"'*) ;;
  *) fail "relative skill-directory target missing" ;;
esac
case "$skill_evidence" in
  *"$temporary_directory/workspace/testdata/skills/example"*)
    fail "workspace-local skill target entered evidence"
    ;;
esac

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
