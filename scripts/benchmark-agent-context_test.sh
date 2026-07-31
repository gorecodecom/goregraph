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

mkdir -p "$temporary_directory/bin" "$temporary_directory/workspace with space"
canonical_workspace=$(cd -P -- "$temporary_directory/workspace with space" && pwd -P)

cat >"$temporary_directory/bin/codex" <<'EOF'
#!/usr/bin/env bash
set -eu
if [ "${1:-}" = "plugin" ] && [ "${2:-}" = "list" ] && [ "${3:-}" = "--json" ]; then
  if [ "${FAKE_REQUIRE_IDENTITY_ARTIFACTS:-0}" = "1" ]; then
    [ "$(cat "$FAKE_OUTPUT/workspace.txt")" = "$FAKE_WORKSPACE" ] || exit 7
    grep -Eq '^[0-9a-f]{64}$' "$FAKE_OUTPUT/workspace.sha256" || exit 7
    [ -s "$FAKE_OUTPUT/codex-args.txt" ] || exit 7
  fi
  [ "${FAKE_PLUGIN_LIST_FAIL:-0}" = "0" ] || exit 9
  printf '[{"id":"workflow-tools","enabled":true}]\n'
  exit 0
fi
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
mutate_workspace=${FAKE_MUTATE_WORKSPACE:-0}
if [ "${FAKE_MUTATE_FINAL_RUN:-0}" = "1" ]; then
  completed_runs=0
  if [ -e "$FAKE_ORDER" ]; then
    completed_runs=$(awk 'END { print NR + 0 }' "$FAKE_ORDER")
  fi
  [ "$completed_runs" -ne 5 ] || mutate_workspace=1
fi
if [ "$mutate_workspace" = "1" ]; then
  printf 'mutation\n' >>"$FAKE_WORKSPACE/service.txt"
fi
case "$prompt" in
  *"Treat source_sections as current source already read"*"run no source-reading commands"*"mark details absent from them as unknown"*)
    printf 'a\n' >>"$FAKE_ORDER"
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
    case "${FAKE_BASELINE_SKILL_READ:-none}" in
      executable)
        emit_command '/bin/cat /opt/codex/plugins/vendor/skills/brainstorming/SKILL.md'
        ;;
      inventory)
        emit_command '/usr/bin/rg --files /opt/codex/plugins/vendor/skills/review'
        ;;
      relative)
        emit_command "/bin/zsh -lc 'cd ../external/skills/review && cat SKILL.md'"
        ;;
      workspace)
        emit_command "/bin/cat '$FAKE_WORKSPACE/testdata/skills/example/SKILL.md'"
        ;;
    esac
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
if [ "$prompt" != "${prompt#*Treat source_sections as current source already read}" ]; then
  if [ -n "${FAKE_ASSISTED_TOKENS:-}" ]; then
    printf '{"type":"turn.completed","usage":{"input_tokens":%s,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}\n' "$FAKE_ASSISTED_TOKENS"
  else
    printf '{"type":"turn.completed","usage":{"input_tokens":170000,"cached_input_tokens":110000,"output_tokens":10000,"reasoning_output_tokens":4000}}\n'
  fi
elif [ -n "${FAKE_BASELINE_TOKENS:-}" ]; then
  printf '{"type":"turn.completed","usage":{"input_tokens":%s,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}\n' "$FAKE_BASELINE_TOKENS"
else
  printf '{"type":"turn.completed","usage":{"input_tokens":190000,"cached_input_tokens":90000,"output_tokens":10000,"reasoning_output_tokens":4000}}\n'
fi
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
Call goregraph context . --query "<focused query>" exactly once before reading indexed source; put the caller's problem statement and requested evidence scope in the query.
Preserve the caller's domain language, identifiers, and requested evidence; exclude workspace setup, tool policy, safety constraints, and output-format instructions. Do not translate or add inferred repository or component responsibilities.
If the context command fails, do not read context-index.json or any generated index; only a missing or stale output error permits goregraph doctor ., otherwise stop using GoreGraph and follow the caller's fallback policy.
Treat source_sections as current source already read; never re-read, grep, or widen an included range.
If source_coverage is complete, run no source-reading commands on indexed project files. Answer only from source_sections and mark details absent from them as unknown.
If source_coverage is partial or none, inspect only exact project/path and start_line/end_line ranges listed in source_omissions; do not inspect outside those ranges or other files. Report pathless or unbounded omissions as uncertainty.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
A missing future call, route, or symbol required by the requested fix is evidence of the current gap, not a source-fallback trigger; assess entrypoint reliability from the existing production path.
For change plans, enumerate exact existing production and test paths supplied by the Context Pack or bounded omission reads, do not invent future filenames, and keep future route, authentication, status, lookup implementation, and cross-service transaction ordering as unknown design decisions unless rendered source proves them.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.
EOF
printf 'fixture\n' >"$canonical_workspace/service.txt"
chmod +x "$temporary_directory/bin/codex" "$temporary_directory/bin/goregraph"

safe_args=$'-a\nnever\nexec\n--sandbox\nread-only\n--skip-git-repo-check\n--ephemeral\n--ignore-user-config\n--ignore-rules\n--color\nnever\n-m\ntest model\n-c\nmodel_reasoning_effort="high"'

: >"$temporary_directory/first-line-only.order"
printf 'Call goregraph context . --query "<focused query>" exactly once before reading indexed source; put the caller'\''s problem statement and requested evidence scope in the query.\n' |
  FAKE_ORDER="$temporary_directory/first-line-only.order" "$temporary_directory/bin/codex" >/dev/null
[ "$(tr -d '\n' <"$temporary_directory/first-line-only.order")" = "b" ] ||
  fail "first-line-only assisted prompt was classified as assisted"

run_harness() {
  result_name=$1
  CODEX_BENCHMARK_ARGS=${2:-$safe_args} \
    PATH="$temporary_directory/bin:$go_bin:/usr/bin:/bin" \
    FAKE_WORKSPACE="$canonical_workspace" \
    FAKE_OUTPUT="$temporary_directory/$result_name" \
    FAKE_REQUIRE_IDENTITY_ARTIFACTS=1 \
    FAKE_ORDER="$temporary_directory/$result_name.order" \
    FAKE_BASELINE_TOKENS=${FAKE_BASELINE_TOKENS:-} \
    FAKE_ASSISTED_TOKENS=${FAKE_ASSISTED_TOKENS:-} \
    FAKE_BASELINE_SKILL_READ=${FAKE_BASELINE_SKILL_READ:-0} \
    FAKE_PLUGIN_LIST_FAIL=${FAKE_PLUGIN_LIST_FAIL:-0} \
    FAKE_ASSISTED_EXTRA_TOOLS=${FAKE_ASSISTED_EXTRA_TOOLS:-0} \
    FAKE_ASSISTED_EXTRA_SOURCE_READS=${FAKE_ASSISTED_EXTRA_SOURCE_READS:-0} \
    FAKE_ASSISTED_COMPACT_DUPLICATE=${FAKE_ASSISTED_COMPACT_DUPLICATE:-0} \
    FAKE_ASSISTED_REPEATED_FULL=${FAKE_ASSISTED_REPEATED_FULL:-0} \
    FAKE_ASSISTED_INCLUDED_REREAD=${FAKE_ASSISTED_INCLUDED_REREAD:-0} \
    FAKE_MUTATE_WORKSPACE=${FAKE_MUTATE_WORKSPACE:-0} \
    FAKE_MUTATE_FINAL_RUN=${FAKE_MUTATE_FINAL_RUN:-0} \
    /bin/bash "$harness" \
      --workspace "$canonical_workspace" \
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
grep -q $'^variant\trun\teffective_tokens\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\texternal_skill_read_calls\ttool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tbounded_omission_read_calls\tunauthorized_source_read_calls\tincluded_source_rereads\tunique_source_files\tlog$' "$temporary_directory/pass/summary.tsv" ||
  fail "summary schema missing"
grep -q $'^baseline\tmedian\t110000\t-\t-\t-\t-\t-\t-\t-\t10\t-\t-\t-\t-\t6\t4\t-\t-\t-\t-\t-$' "$temporary_directory/pass/summary.tsv" ||
  fail "baseline median missing"
grep -q $'^assisted\tmedian\t70000\t-\t-\t-\t-\t-\t-\t-\t6\t-\t-\t-\t-\t2\t2\t-\t-\t-\t-\t-$' "$temporary_directory/pass/summary.tsv" ||
  fail "assisted median missing"
[ -s "$temporary_directory/pass/codex-plugins.json" ] ||
  fail "Codex plugin inventory was not retained"
[ -s "$temporary_directory/pass/assisted-1.log.metrics.tsv" ] ||
  fail "analyzer result was not retained"
[ -s "$temporary_directory/pass/assisted-1.log.stderr" ] ||
  fail "Codex stderr was not retained separately"
[ "$(cat "$temporary_directory/pass/workspace.txt")" = "$canonical_workspace" ] ||
  fail "canonical benchmark workspace was not retained"
grep -Eq '^[0-9a-f]{64}$' "$temporary_directory/pass/workspace.sha256" ||
  fail "workspace snapshot identity was not retained"
expected_codex_args="$temporary_directory/expected-codex-args.txt"
printf '%s\n' \
  '-a' \
  'never' \
  'exec' \
  '--sandbox' \
  'read-only' \
  '--skip-git-repo-check' \
  '--ephemeral' \
  '--ignore-user-config' \
  '--ignore-rules' \
  '--color' \
  'never' \
  '-m' \
  'test model' \
  '-c' \
  'model_reasoning_effort="high"' \
  '--json' \
  '-C' \
  "$canonical_workspace" \
  '-' >"$expected_codex_args"
cmp -s "$expected_codex_args" "$temporary_directory/pass/codex-args.txt" ||
  fail "complete effective Codex argument vector was not retained"

mkfifo "$canonical_workspace/unsupported.pipe"
if run_harness workspace-identity-failure >/dev/null 2>&1; then
  fail "unsupported workspace identity passed"
fi
rm -f -- "$canonical_workspace/unsupported.pipe"
[ ! -s "$temporary_directory/workspace-identity-failure.order" ] ||
  fail "Codex run started after workspace identity failure"
[ ! -e "$temporary_directory/workspace-identity-failure/codex-plugins.json" ] ||
  fail "Codex plugin inventory started before workspace identity was established"

FAKE_MUTATE_WORKSPACE=1
export FAKE_MUTATE_WORKSPACE
if run_harness workspace-mutated >/dev/null 2>&1; then
  fail "workspace mutation passed"
fi
unset FAKE_MUTATE_WORKSPACE
printf 'fixture\n' >"$canonical_workspace/service.txt"
[ "$(tr -d '\n' <"$temporary_directory/workspace-mutated.order")" = "b" ] ||
  fail "harness started another run after workspace mutation"
[ ! -e "$temporary_directory/workspace-mutated/assisted-1.log" ] ||
  fail "assisted run started after workspace mutation"

FAKE_MUTATE_FINAL_RUN=1
export FAKE_MUTATE_FINAL_RUN
if run_harness final-workspace-mutated \
  >"$temporary_directory/final-workspace-mutated.stdout" \
  2>"$temporary_directory/final-workspace-mutated.stderr"; then
  fail "final workspace mutation passed"
fi
unset FAKE_MUTATE_FINAL_RUN
printf 'fixture\n' >"$canonical_workspace/service.txt"
[ "$(tr -d '\n' <"$temporary_directory/final-workspace-mutated.order")" = "baabba" ] ||
  fail "final workspace mutation did not occur during the sixth run"
[ -s "$temporary_directory/final-workspace-mutated/assisted-3.log" ] ||
  fail "final mutated run transcript was not retained"
[ -s "$temporary_directory/final-workspace-mutated/assisted-3.log.stderr" ] ||
  fail "final mutated run stderr was not retained"
[ ! -e "$temporary_directory/final-workspace-mutated/assisted-3.log.metrics.tsv" ] ||
  fail "final mutated run was analyzed"
[ ! -e "$temporary_directory/final-workspace-mutated/assisted-3.log.skill-reads.json" ] ||
  fail "final mutated run produced skill-read evidence"
run_rows=$(awk -F '\t' '$2 ~ /^[1-3]$/ { rows++ } END { print rows + 0 }' \
  "$temporary_directory/final-workspace-mutated/summary.tsv")
[ "$run_rows" -eq 5 ] || fail "final workspace mutation retained $run_rows run rows, want 5"
if grep -q $'\tmedian\t' "$temporary_directory/final-workspace-mutated/summary.tsv"; then
  fail "final workspace mutation produced aggregate median evidence"
fi
if grep -q 'Token and structural gates passed' \
  "$temporary_directory/final-workspace-mutated.stdout"; then
  fail "final workspace mutation produced a passing aggregate verdict"
fi
grep -q 'workspace snapshot changed' "$temporary_directory/final-workspace-mutated.stderr" ||
  fail "final workspace mutation did not report the snapshot failure"

FAKE_ASSISTED_TOKENS=88001
export FAKE_ASSISTED_TOKENS
if run_harness over-eighty >/dev/null 2>&1; then
  fail "80% plus one token passed"
fi
grep -q $'^assisted\tmedian\t88001\t' "$temporary_directory/over-eighty/summary.tsv" ||
  fail "failed gate did not retain median evidence"
unset FAKE_ASSISTED_TOKENS

FAKE_BASELINE_TOKENS=200000
FAKE_ASSISTED_TOKENS=116561
export FAKE_BASELINE_TOKENS FAKE_ASSISTED_TOKENS
if run_harness over-absolute-cap >/dev/null 2>&1; then
  fail "116561 assisted effective tokens passed the absolute cap"
fi
grep -q $'^assisted\tmedian\t116561\t' \
  "$temporary_directory/over-absolute-cap/summary.tsv" ||
  fail "absolute-cap failure did not retain assisted effective-token evidence"
unset FAKE_BASELINE_TOKENS FAKE_ASSISTED_TOKENS

for contamination_mode in executable inventory relative; do
  result_name="contaminated-$contamination_mode"
  FAKE_BASELINE_SKILL_READ=$contamination_mode
  export FAKE_BASELINE_SKILL_READ
  if run_harness "$result_name" >/dev/null 2>&1; then
    fail "$contamination_mode contaminated baseline passed"
  fi
  unset FAKE_BASELINE_SKILL_READ
  [ "$(tr -d '\n' <"$temporary_directory/$result_name.order")" = "b" ] ||
    fail "harness did not stop after first $contamination_mode contaminated run"
  awk -F '\t' '$1 == "baseline" && $2 == "1" { found = 1; if ($10 != 1) exit 1 } END { if (!found) exit 1 }' \
    "$temporary_directory/$result_name/summary.tsv" ||
    fail "$contamination_mode contaminated run was not retained in summary"
  [ -s "$temporary_directory/$result_name/baseline-1.log.skill-reads.json" ] ||
    fail "$contamination_mode ordered skill evidence was not retained"
  [ ! -e "$temporary_directory/$result_name/assisted-1.log" ] ||
    fail "harness launched a run after $contamination_mode contamination"
done

FAKE_BASELINE_SKILL_READ=workspace
export FAKE_BASELINE_SKILL_READ
run_harness workspace-skill >/dev/null
unset FAKE_BASELINE_SKILL_READ
[ "$(tr -d '\n' <"$temporary_directory/workspace-skill.order")" = "baabba" ] ||
  fail "workspace-local skill path stopped the matrix"
awk -F '\t' '$1 == "baseline" && $2 ~ /^[1-3]$/ { found++; if ($10 != 0) exit 1 } END { if (found != 3) exit 1 }' \
  "$temporary_directory/workspace-skill/summary.tsv" ||
  fail "workspace-local skill path was classified as external"
cmp -s "$temporary_directory/pass/workspace.sha256" \
  "$temporary_directory/workspace-skill/workspace.sha256" ||
  fail "identical workspace snapshots produced different identities"

FAKE_PLUGIN_LIST_FAIL=1
export FAKE_PLUGIN_LIST_FAIL
if run_harness plugin-inventory-failure >/dev/null 2>&1; then
  fail "missing plugin inventory passed"
fi
unset FAKE_PLUGIN_LIST_FAIL
[ ! -s "$temporary_directory/plugin-inventory-failure.order" ] ||
  fail "Codex run started after plugin inventory failure"

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

controlled_skill_config='skills.config=[{path="/opt/superpowers/using-superpowers/SKILL.md",enabled=false},{path="/opt/superpowers/systematic-debugging/SKILL.md",enabled=false}]'
controlled_skill_args="${safe_args}"$'\n-c\n'"$controlled_skill_config"
run_harness controlled-skill-config "$controlled_skill_args" >/dev/null
grep -Fqx -- "$controlled_skill_config" \
  "$temporary_directory/controlled-skill-config/codex-args.txt" ||
  fail "controlled skill config was not retained losslessly"

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

blank_model_args=${safe_args/test model/   }
if run_harness blank-model "$blank_model_args" >/dev/null 2>&1; then
  fail "blank model setting passed"
fi
[ ! -s "$temporary_directory/blank-model.order" ] ||
  fail "Codex ran before blank model rejection"

sentinel="$temporary_directory/injected"
literal_argument="\$(touch \"$sentinel\")"
literal_args=${safe_args/test model/$literal_argument}
case "$literal_args" in
  *"$literal_argument"*) ;;
  *) fail "literal argument fixture does not contain the injection payload" ;;
esac
run_harness literal-argument "$literal_args" >/dev/null
[ ! -e "$sentinel" ] || fail "literal Codex argument executed shell text"
grep -Fqx -- "$literal_argument" "$temporary_directory/literal-argument/codex-args.txt" ||
  fail "literal Codex argument was not retained losslessly"

printf 'PASS: benchmark-agent-context harness\n'
