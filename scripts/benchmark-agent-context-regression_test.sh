#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
repo_root=$(cd -P -- "$script_dir/.." && pwd -P)
runner="$script_dir/benchmark-agent-context-regression.sh"
matrix="$repo_root/testdata/agent-context-regression/matrix.json"

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-regression-test.XXXXXX")
cleanup() {
  status=$?
  trap - EXIT
  rm -rf -- "$temporary_directory"
  exit "$status"
}
trap cleanup EXIT

external_case="$temporary_directory/g1"
mkdir -p -- "$external_case"
cp -R -- "$repo_root/testdata/agent-context-regression/g3-go-existing-flow/workspace" \
  "$external_case/workspace"
sed 's/g3-go-existing-flow/g1/g' \
  "$repo_root/testdata/agent-context-regression/g3-go-existing-flow/contract.json" \
  >"$external_case/contract.json"
cp -- "$repo_root/testdata/agent-context-regression/g3-go-existing-flow/query.en.txt" \
  "$external_case/query.en.txt"
printf 'Use the prepared Context Pack and answer only from bounded evidence.\n' \
  >"$temporary_directory/instruction.txt"

fake_bin="$temporary_directory/bin"
mkdir -- "$fake_bin"
process_log="$temporary_directory/process.log"
: >"$process_log"
codex_counter="$temporary_directory/codex-counter"

write_goregraph_fake() {
  path=$1
  build=$2
  sed "s/__BUILD__/$build/g" >"$path" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "version" ]; then
  printf '__BUILD__\n'
  exit 0
fi
case "${1:-}" in
  workspace)
    workspace=${3:-}
    printf 'scan\t__BUILD__\t%s\n' "$workspace" >>"$FAKE_RUNNER_LOG"
    mkdir -p -- "$workspace/.goregraph-workspace/agent"
    printf '{"schema":1,"build":"__BUILD__"}\n' \
      >"$workspace/.goregraph-workspace/agent/context-index.json"
    ;;
  context)
    printf 'context\t__BUILD__\t%s\n' "${2:-}" >>"$FAKE_RUNNER_LOG"
    printf '{"schema":1,"query":"fake","confidence":"high","fallback_required":false,"estimated_tokens":10,"budget_tokens":4000,"retry_allowed":false}\n'
    ;;
  *)
    printf 'unexpected fake goregraph arguments: %s\n' "$*" >&2
    exit 9
    ;;
esac
EOF
  chmod 700 "$path"
}

write_goregraph_fake "$fake_bin/golden" golden
write_goregraph_fake "$fake_bin/candidate" candidate

cat >"$fake_bin/codex" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "--version" ]; then
  printf 'codex-fake 1.0\n'
  exit 0
fi
workspace=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "-C" ]; then
    workspace=$argument
  fi
  previous=$argument
done
build=$(goregraph version)
printf 'codex\t%s\t%s\t%s\n' "$build" "$workspace" "$(command -v goregraph)" \
  >>"$FAKE_RUNNER_LOG"
cat >/dev/null
count=0
if [ -f "$FAKE_CODEX_COUNTER" ]; then
  count=$(cat "$FAKE_CODEX_COUNTER")
fi
printf '%s\n' "$((count + 1))" >"$FAKE_CODEX_COUNTER"
printf '{"type":"item.completed","item":{"id":"tool-1","type":"command_execution","command":"pwd"}}\n'
printf '{"type":"turn.completed","usage":{"total_tokens":100}}\n'
EOF
chmod 700 "$fake_bin/codex"

safe_args=$'-a\nnever\nexec\n--sandbox\nread-only\n--skip-git-repo-check\n--ephemeral\n--ignore-user-config\n--ignore-rules\n--color\nnever\n-m\ntest-model\n-c\nmodel_reasoning_effort="high"'
output="$temporary_directory/output"

PATH="$fake_bin:$PATH" \
FAKE_RUNNER_LOG="$process_log" \
FAKE_CODEX_COUNTER="$codex_counter" \
CODEX_BENCHMARK_ARGS="$safe_args" \
GOCACHE="$temporary_directory/go-cache" \
"$runner" run \
  --matrix "$matrix" \
  --external-case "$external_case" \
  --golden-binary "$fake_bin/golden" \
  --golden-commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  --candidate-binary "$fake_bin/candidate" \
  --candidate-commit bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
  --instruction "$temporary_directory/instruction.txt" \
  --phase smoke \
  --target-case g1 \
  --runs 1 \
  --output "$output"

[ "$(cat "$codex_counter")" = "2" ] || {
  printf 'FAIL: Codex execution count is %s, want 2\n' "$(cat "$codex_counter")" >&2
  exit 1
}
grep -q $'^codex\tgolden\t' "$process_log"
grep -q $'^codex\tcandidate\t' "$process_log"

for artifact in \
  inputs/matrix.json \
  inputs/external-case/contract.json \
  inputs/instruction.txt \
  inputs/prompt-digests.tsv \
  identity/run-order.tsv \
  identity/codex-args.txt \
  cases/g1/english/golden-pack.json \
  cases/g1/english/candidate-pack.json \
  cases/g1/english/pack-diff.json \
  runs/g1/english/golden-1-1.jsonl \
  runs/g1/english/candidate-1-1.metrics.tsv \
  reviews/g1/english/golden-1-1.json \
  summary.tsv; do
  [ -f "$output/$artifact" ] || {
    printf 'FAIL: missing artifact %s\n' "$artifact" >&2
    exit 1
  }
done

if PATH="$fake_bin:$PATH" \
  FAKE_RUNNER_LOG="$process_log" \
  FAKE_CODEX_COUNTER="$codex_counter" \
  CODEX_BENCHMARK_ARGS="$safe_args" \
  GOCACHE="$temporary_directory/go-cache" \
  "$runner" run \
    --matrix "$matrix" \
    --external-case "$external_case" \
    --golden-binary "$fake_bin/golden" \
    --golden-commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
    --candidate-binary "$fake_bin/candidate" \
    --candidate-commit bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
    --instruction "$temporary_directory/instruction.txt" \
    --phase smoke \
    --target-case g1 \
    --runs 1 \
    --output "$output" >/dev/null 2>&1; then
  printf 'FAIL: existing output was accepted\n' >&2
  exit 1
fi

printf 'PASS: benchmark-agent-context-regression\n'
