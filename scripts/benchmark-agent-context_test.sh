#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
harness="$script_dir/benchmark-agent-context.sh"
go_bin=$(dirname -- "$(command -v go)")
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-benchmark-test.XXXXXX")
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

mkdir -p "$temporary_directory/bin" "$temporary_directory/workspace"

cat >"$temporary_directory/bin/codex" <<'EOF'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "--version" ]; then
  printf 'codex-test 1.0\n'
  exit 0
fi
if [ "$#" -gt 0 ]; then
  json_count=0
  for argument in "$@"; do
    [ "$argument" = "--json" ] && json_count=$((json_count + 1))
  done
  [ "$json_count" -eq 1 ] || exit 4
  printf 'codex-test warning\n' >&2
fi
event_number=0
emit_command() {
  event_number=$((event_number + 1))
  if [ "$#" -eq 2 ]; then
    printf '{"type":"item.completed","item":{"id":"command-%s","type":"command_execution","command":"%s","exit_code":0,"aggregated_output":"%s"}}\n' "$event_number" "$1" "$2"
  else
    printf '{"type":"item.completed","item":{"id":"command-%s","type":"command_execution","command":"%s","exit_code":0}}\n' "$event_number" "$1"
  fi
}
prompt=$(cat)
case "$prompt" in
  *"Treat source_sections as current source already read"*"run no source-reading commands"*"mark details absent from them as unknown"*)
    printf 'a\n' >>"$FAKE_ORDER"
    tokens=${FAKE_ASSISTED_TOKENS:-80000}
    context_pack='# Context Pack\n\nContext ID: assisted-pack\n'
    if [ "${FAKE_ASSISTED_INCLUDED_REREAD:-0}" = "1" ]; then
      context_pack='# Context Pack\n\nContext ID: assisted-pack\nSource coverage: complete\n\n## Source sections\n\n### 1. `src/Service.java:1-40`\n'
    fi
    emit_command 'goregraph context /work --query route' "$context_pack"
    emit_command 'sed -n 1,40p /work/src/Service.java'
    emit_command 'sed -n 41,80p /work/src/Worker.go'
    emit_command 'make test'
    emit_command 'git status'
    emit_command 'pwd'
    extra_tools=${FAKE_ASSISTED_EXTRA_TOOLS:-0}
    while [ "$extra_tools" -gt 0 ]; do
      emit_command 'make lint'
      extra_tools=$((extra_tools - 1))
    done
    extra_source_reads=${FAKE_ASSISTED_EXTRA_SOURCE_READS:-0}
    while [ "$extra_source_reads" -gt 0 ]; do
      emit_command "cat /work/src/Extra${extra_source_reads}.py"
      extra_source_reads=$((extra_source_reads - 1))
    done
    if [ "${FAKE_ASSISTED_COMPACT_DUPLICATE:-0}" = "1" ]; then
      emit_command 'goregraph context /work --query retry' '# Context Pack\n\nContext ID: compact-pack\nDuplicate of: assisted-pack\n'
    fi
    if [ "${FAKE_ASSISTED_REPEATED_FULL:-0}" = "1" ]; then
      emit_command 'goregraph context /work --query retry' '# Context Pack\n\nContext ID: assisted-pack\n'
    fi
    ;;
  *)
    printf 'b\n' >>"$FAKE_ORDER"
    tokens=${FAKE_BASELINE_TOKENS:-100000}
    if [ "${FAKE_BASELINE_ZERO_SOURCE_READS:-0}" = "1" ]; then
      for number in 1 2 3 4 5 6 7 8 9 10; do
        emit_command 'make test'
      done
    else
      emit_command 'rg -n Service /work/src/Service.java'
      emit_command 'grep -n Worker /work/src/Worker.go'
      emit_command 'sed -n 1,40p /work/src/Service.java'
      emit_command 'sed -n 41,80p /work/src/Service.java'
      emit_command 'nl -ba /work/src/Worker.go'
      emit_command 'cat /work/src/Worker.go'
      emit_command 'make test'
      emit_command 'git status'
      emit_command 'pwd'
      emit_command 'go test ./...'
    fi
    ;;
esac
printf '{"type":"turn.completed","usage":{"input_tokens":%s,"cached_input_tokens":0,"output_tokens":0,"total_tokens":%s}}\n' "$tokens" "$tokens"
EOF

cat >"$temporary_directory/bin/goregraph" <<'EOF'
#!/usr/bin/env bash
set -eu
case "${1:-}" in
  version) printf 'goregraph 1.3.0\n' ;;
  context) printf '{"schema":2,"fallback_required":false}\n' ;;
  *) exit 2 ;;
esac
EOF

cat >"$temporary_directory/base-prompt.txt" <<'EOF'
Inspect the prepared services and explain the requested implementation.
EOF
cat >"$temporary_directory/baseline-instruction.txt" <<'EOF'
Do not use the goregraph CLI, MCP tools, goregraph-out, or .goregraph-workspace files.
EOF
cat >"$temporary_directory/assisted-instruction.txt" <<'EOF'
Call goregraph context once with the complete task before reading indexed source.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
EOF
printf 'fixture\n' >"$temporary_directory/workspace/service.txt"
chmod +x "$temporary_directory/bin/codex" "$temporary_directory/bin/goregraph"

safe_args=$'-a\nnever\nexec\n--sandbox\nread-only\n--skip-git-repo-check\n--ephemeral\n--ignore-user-config\n--ignore-rules\n--color\nnever\n-m\ntest-model\n-c\nmodel_reasoning_effort="high"'

: >"$temporary_directory/first-line-only.order"
printf 'Call goregraph context once with the complete task before reading indexed source.\n' |
  FAKE_ORDER="$temporary_directory/first-line-only.order" "$temporary_directory/bin/codex" >/dev/null
[ "$(tr -d '\n' <"$temporary_directory/first-line-only.order")" = "b" ] ||
  fail "first-line-only assisted prompt was classified as assisted"

run_harness() {
  result_name=$1
  CODEX_BENCHMARK_ARGS=${2:-$safe_args} \
    PATH="$temporary_directory/bin:$go_bin:/usr/bin:/bin" \
    FAKE_ORDER="$temporary_directory/$result_name.order" \
    FAKE_BASELINE_TOKENS=${FAKE_BASELINE_TOKENS:-100000} \
    FAKE_ASSISTED_TOKENS=${FAKE_ASSISTED_TOKENS:-80000} \
    FAKE_ASSISTED_EXTRA_TOOLS=${FAKE_ASSISTED_EXTRA_TOOLS:-0} \
    FAKE_ASSISTED_EXTRA_SOURCE_READS=${FAKE_ASSISTED_EXTRA_SOURCE_READS:-0} \
    FAKE_ASSISTED_COMPACT_DUPLICATE=${FAKE_ASSISTED_COMPACT_DUPLICATE:-0} \
    FAKE_ASSISTED_REPEATED_FULL=${FAKE_ASSISTED_REPEATED_FULL:-0} \
    FAKE_ASSISTED_INCLUDED_REREAD=${FAKE_ASSISTED_INCLUDED_REREAD:-0} \
    /bin/bash "$harness" \
      --workspace "$temporary_directory/workspace" \
      --prompt "$temporary_directory/base-prompt.txt" \
      --baseline-instruction "${BASELINE_INSTRUCTION:-$temporary_directory/baseline-instruction.txt}" \
      --assisted-instruction "${ASSISTED_INSTRUCTION:-$temporary_directory/assisted-instruction.txt}" \
      --runs 3 \
      --output "$temporary_directory/$result_name"
}

/bin/bash -n "$harness"

run_harness pass >/dev/null
actual_order=$(tr -d '\n' <"$temporary_directory/pass.order")
[ "$actual_order" = "baabba" ] || fail "run order = $actual_order, want baabba"
grep -q $'^variant\trun\ttokens\ttool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tincluded_source_rereads\tunique_source_files\tlog$' "$temporary_directory/pass/summary.tsv" ||
  fail "summary schema missing"
grep -q $'^baseline\tmedian\t100000\t10\t-\t-\t-\t-\t6\t4\t-\t-\t-$' "$temporary_directory/pass/summary.tsv" ||
  fail "baseline median missing"
grep -q $'^assisted\tmedian\t80000\t6\t-\t-\t-\t-\t2\t2\t-\t-\t-$' "$temporary_directory/pass/summary.tsv" ||
  fail "assisted median missing"
[ -s "$temporary_directory/pass/assisted-1.log.metrics.tsv" ] ||
  fail "analyzer result was not retained"
[ -s "$temporary_directory/pass/assisted-1.log.stderr" ] ||
  fail "Codex stderr was not retained separately"

FAKE_ASSISTED_TOKENS=80001
export FAKE_ASSISTED_TOKENS
if run_harness over-eighty >/dev/null 2>&1; then
  fail "80% plus one token passed"
fi
grep -q $'^assisted\tmedian\t80001\t6\t-\t-\t-\t-\t2\t2\t-\t-\t-$' "$temporary_directory/over-eighty/summary.tsv" ||
  fail "failed gate did not retain median evidence"
unset FAKE_ASSISTED_TOKENS

FAKE_ASSISTED_EXTRA_TOOLS=2
export FAKE_ASSISTED_EXTRA_TOOLS
if run_harness over-tool-gate >/dev/null 2>&1; then
  fail "tool-call gate passed"
fi
unset FAKE_ASSISTED_EXTRA_TOOLS

FAKE_ASSISTED_EXTRA_SOURCE_READS=1
export FAKE_ASSISTED_EXTRA_SOURCE_READS
if run_harness over-source-read-gate >/dev/null 2>&1; then
  fail "source-read gate passed"
fi
unset FAKE_ASSISTED_EXTRA_SOURCE_READS

FAKE_ASSISTED_INCLUDED_REREAD=1
export FAKE_ASSISTED_INCLUDED_REREAD
if run_harness included-source-reread >/dev/null 2>&1; then
  fail "included source reread passed"
fi
unset FAKE_ASSISTED_INCLUDED_REREAD

FAKE_ASSISTED_COMPACT_DUPLICATE=1
export FAKE_ASSISTED_COMPACT_DUPLICATE
run_harness compact-duplicate >/dev/null
unset FAKE_ASSISTED_COMPACT_DUPLICATE

FAKE_ASSISTED_REPEATED_FULL=1
export FAKE_ASSISTED_REPEATED_FULL
if run_harness repeated-full-pack >/dev/null 2>&1; then
  fail "repeated full Context Pack passed"
fi
unset FAKE_ASSISTED_REPEATED_FULL

FAKE_BASELINE_ZERO_SOURCE_READS=1
export FAKE_BASELINE_ZERO_SOURCE_READS
if run_harness zero-baseline-source-reads >/dev/null 2>&1; then
  fail "zero baseline source reads passed"
fi
unset FAKE_BASELINE_ZERO_SOURCE_READS

cp "$temporary_directory/assisted-instruction.txt" "$temporary_directory/assisted-extra-newlines.txt"
printf '\n\n' >>"$temporary_directory/assisted-extra-newlines.txt"
ASSISTED_INSTRUCTION="$temporary_directory/assisted-extra-newlines.txt"
export ASSISTED_INSTRUCTION
if run_harness extra-newlines >/dev/null 2>&1; then
  fail "instruction with extra newlines passed"
fi
[ ! -s "$temporary_directory/extra-newlines.order" ] ||
  fail "Codex ran before instruction rejection"
unset ASSISTED_INSTRUCTION

FAKE_BASELINE_TOKENS=1,0,0,0,0
export FAKE_BASELINE_TOKENS
if run_harness malformed-tokens >/dev/null 2>&1; then
  fail "malformed token grouping passed"
fi
unset FAKE_BASELINE_TOKENS

unsafe_args="${safe_args}"$'\n-c\nfeatures.web_search=true'
if run_harness unsafe-config "$unsafe_args" >/dev/null 2>&1; then
  fail "unsafe config override passed"
fi
[ ! -s "$temporary_directory/unsafe-config.order" ] ||
  fail "Codex ran before unsafe config rejection"

json_args="${safe_args}"$'\n--json'
if run_harness user-json "$json_args" >/dev/null 2>&1; then
  fail "user-supplied JSON mode passed"
fi
[ ! -s "$temporary_directory/user-json.order" ] ||
  fail "Codex ran before JSON mode rejection"

empty_reasoning_args=${safe_args/model_reasoning_effort=\"high\"/model_reasoning_effort=}
if run_harness empty-reasoning "$empty_reasoning_args" >/dev/null 2>&1; then
  fail "empty reasoning setting passed"
fi
[ ! -s "$temporary_directory/empty-reasoning.order" ] ||
  fail "Codex ran before empty reasoning rejection"

blank_model_args=${safe_args/test-model/   }
if run_harness blank-model "$blank_model_args" >/dev/null 2>&1; then
  fail "blank model setting passed"
fi
[ ! -s "$temporary_directory/blank-model.order" ] ||
  fail "Codex ran before blank model rejection"

sentinel="$temporary_directory/injected"
literal_args=${safe_args/test-model/\$\(touch "$sentinel"\)}
run_harness literal-argument "$literal_args" >/dev/null
[ ! -e "$sentinel" ] || fail "literal Codex argument executed shell text"

printf 'PASS: benchmark-agent-context harness\n'
