#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
harness="$script_dir/benchmark-agent-context.sh"
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
prompt=$(cat)
case "$prompt" in
  *"Treat source_sections as current source already read"*)
    printf 'a\n' >>"$FAKE_ORDER"
    tokens=${FAKE_ASSISTED_TOKENS:-80.000}
    printf '{"type":"tool_call","tool_name":"exec","command":"goregraph context /work --query route"}\n'
    printf '{"type":"tool_call","tool_name":"exec","command":"sed -n 1,40p /work/src/Service.java"}\n'
    printf '{"type":"tool_call","tool_name":"exec","command":"sed -n 41,80p /work/src/Worker.go"}\n'
    printf '{"type":"tool_call","tool_name":"exec","command":"make test"}\n'
    printf '{"type":"tool_call","tool_name":"exec","command":"git status"}\n'
    printf '{"type":"tool_call","tool_name":"exec","command":"pwd"}\n'
    extra_tools=${FAKE_ASSISTED_EXTRA_TOOLS:-0}
    while [ "$extra_tools" -gt 0 ]; do
      printf '{"type":"tool_call","tool_name":"exec","command":"make lint"}\n'
      extra_tools=$((extra_tools - 1))
    done
    extra_source_reads=${FAKE_ASSISTED_EXTRA_SOURCE_READS:-0}
    while [ "$extra_source_reads" -gt 0 ]; do
      printf '{"type":"tool_call","tool_name":"exec","command":"cat /work/src/Extra%s.py"}\n' "$extra_source_reads"
      extra_source_reads=$((extra_source_reads - 1))
    done
    printf '{"context_id":"assisted-pack"}\n'
    if [ "${FAKE_ASSISTED_DUPLICATE_PACK:-0}" = "1" ]; then
      printf '{"context_id":"retry-pack","duplicate_of":"assisted-pack"}\n'
    fi
    ;;
  *)
    printf 'b\n' >>"$FAKE_ORDER"
    tokens=${FAKE_BASELINE_TOKENS:-100,000}
    if [ "${FAKE_BASELINE_ZERO_SOURCE_READS:-0}" = "1" ]; then
      for number in 1 2 3 4 5 6 7 8 9 10; do
        printf '{"type":"tool_call","tool_name":"exec","command":"make test"}\n'
      done
    else
      printf '{"type":"tool_call","tool_name":"exec","command":"rg -n Service /work/src/Service.java"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"grep -n Worker /work/src/Worker.go"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"sed -n 1,40p /work/src/Service.java"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"sed -n 41,80p /work/src/Service.java"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"nl -ba /work/src/Worker.go"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"cat /work/src/Worker.go"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"make test"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"git status"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"pwd"}\n'
      printf '{"type":"tool_call","tool_name":"exec","command":"go test ./..."}\n'
    fi
    ;;
esac
printf 'tokens used\n%s\n' "$tokens"
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
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, read only relevant uncovered ranges named by source_omissions or files not represented by source_sections.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
At most one narrower retry may use an exact route, qualified symbol, or file returned by the first call; never use a call-chain label.
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
    PATH="$temporary_directory/bin:/usr/bin:/bin" \
    FAKE_ORDER="$temporary_directory/$result_name.order" \
    FAKE_BASELINE_TOKENS=${FAKE_BASELINE_TOKENS:-100,000} \
    FAKE_ASSISTED_TOKENS=${FAKE_ASSISTED_TOKENS:-80.000} \
    FAKE_ASSISTED_EXTRA_TOOLS=${FAKE_ASSISTED_EXTRA_TOOLS:-0} \
    FAKE_ASSISTED_EXTRA_SOURCE_READS=${FAKE_ASSISTED_EXTRA_SOURCE_READS:-0} \
    FAKE_ASSISTED_DUPLICATE_PACK=${FAKE_ASSISTED_DUPLICATE_PACK:-0} \
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
grep -q $'^variant\trun\ttokens\ttool_calls\tgoregraph_calls\tfull_context_packs\tduplicate_context_packs\traw_navigation_calls\tsource_read_calls\tunique_source_files\tlog$' "$temporary_directory/pass/summary.tsv" ||
  fail "summary schema missing"
grep -q $'^baseline\tmedian\t100000\t10\t-\t-\t-\t6\t4\t-\t-$' "$temporary_directory/pass/summary.tsv" ||
  fail "baseline median missing"
grep -q $'^assisted\tmedian\t80000\t6\t-\t-\t-\t2\t2\t-\t-$' "$temporary_directory/pass/summary.tsv" ||
  fail "assisted median missing"
[ -s "$temporary_directory/pass/assisted-1.log.metrics.tsv" ] ||
  fail "analyzer result was not retained"

FAKE_ASSISTED_TOKENS=80.001
export FAKE_ASSISTED_TOKENS
if run_harness over-eighty >/dev/null 2>&1; then
  fail "80% plus one token passed"
fi
grep -q $'^assisted\tmedian\t80001\t6\t-\t-\t-\t2\t2\t-\t-$' "$temporary_directory/over-eighty/summary.tsv" ||
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

FAKE_ASSISTED_DUPLICATE_PACK=1
export FAKE_ASSISTED_DUPLICATE_PACK
if run_harness duplicate-pack-gate >/dev/null 2>&1; then
  fail "duplicate Context Pack gate passed"
fi
unset FAKE_ASSISTED_DUPLICATE_PACK

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
