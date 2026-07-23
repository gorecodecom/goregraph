#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
analyzer="$script_dir/analyze-agent-context-log.sh"
analyzer_go="$script_dir/analyze-agent-context-log.go"

usage() {
  cat <<'EOF'
Usage: scripts/benchmark-agent-context.sh \
  --workspace /absolute/prepared-workspace \
  --prompt /absolute/base-prompt.txt \
  --baseline-instruction /absolute/baseline-instruction.txt \
  --assisted-instruction /absolute/context-instruction.txt \
  --runs 3 \
  --output /absolute/new-results-directory

CODEX_BENCHMARK_ARGS must contain one literal Codex argument per line.
EOF
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

require_absolute() {
  case "$2" in
    /*) ;;
    *) die "$1 must be an absolute path: $2" ;;
  esac
  case "$2" in
    *$'\t'*|*$'\n'*) die "$1 must not contain tabs or newlines" ;;
  esac
}

canonical_directory() {
  (cd -P -- "$1" 2>/dev/null && pwd -P) || return 1
}

canonical_file() {
  file_dir=$(dirname -- "$1")
  file_name=$(basename -- "$1")
  canonical_dir=$(canonical_directory "$file_dir") || return 1
  printf '%s/%s\n' "$canonical_dir" "$file_name"
}

is_reasoning_config() {
  case "$1" in
    model_reasoning_effort=minimal|model_reasoning_effort=low|model_reasoning_effort=medium|model_reasoning_effort=high|model_reasoning_effort=xhigh)
      return 0
      ;;
    model_reasoning_effort=\"minimal\"|model_reasoning_effort=\"low\"|model_reasoning_effort=\"medium\"|model_reasoning_effort=\"high\"|model_reasoning_effort=\"xhigh\")
      return 0
      ;;
  esac
  return 1
}

require_nonblank_model() {
  case "$1" in
    *[![:space:]]*) return 0 ;;
  esac
  die "model must not be empty or blank"
}

workspace=""
prompt=""
baseline_instruction=""
assisted_instruction=""
runs=""
output=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --workspace|--prompt|--baseline-instruction|--assisted-instruction|--runs|--output)
      [ "$#" -ge 2 ] || die "$1 requires a value"
      option=$1
      value=$2
      shift 2
      case "$option" in
        --workspace) [ -z "$workspace" ] || die "duplicate --workspace"; workspace=$value ;;
        --prompt) [ -z "$prompt" ] || die "duplicate --prompt"; prompt=$value ;;
        --baseline-instruction) [ -z "$baseline_instruction" ] || die "duplicate --baseline-instruction"; baseline_instruction=$value ;;
        --assisted-instruction) [ -z "$assisted_instruction" ] || die "duplicate --assisted-instruction"; assisted_instruction=$value ;;
        --runs) [ -z "$runs" ] || die "duplicate --runs"; runs=$value ;;
        --output) [ -z "$output" ] || die "duplicate --output"; output=$value ;;
      esac
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

[ -n "$workspace" ] || die "--workspace is required"
[ -n "$prompt" ] || die "--prompt is required"
[ -n "$baseline_instruction" ] || die "--baseline-instruction is required"
[ -n "$assisted_instruction" ] || die "--assisted-instruction is required"
[ -n "$runs" ] || die "--runs is required"
[ -n "$output" ] || die "--output is required"

case "$runs" in
  *[!0-9]*|"") die "--runs must be an odd integer of at least 3" ;;
esac
[ "$runs" -ge 3 ] || die "--runs must be an odd integer of at least 3"
[ $((runs % 2)) -eq 1 ] || die "--runs must be an odd integer of at least 3"

for command_name in codex goregraph go awk sed sort grep mktemp cat cp cmp mkdir rm dirname basename; do
  require_command "$command_name"
done
[ -f "$analyzer" ] && [ -r "$analyzer" ] ||
  die "transcript analyzer is not a readable regular file: $analyzer"
[ -f "$analyzer_go" ] && [ -r "$analyzer_go" ] ||
  die "transcript analyzer helper is not a readable regular file: $analyzer_go"

require_absolute "--workspace" "$workspace"
require_absolute "--prompt" "$prompt"
require_absolute "--baseline-instruction" "$baseline_instruction"
require_absolute "--assisted-instruction" "$assisted_instruction"
require_absolute "--output" "$output"

workspace=$(canonical_directory "$workspace") || die "workspace is not a readable directory: $workspace"
for input_path in "$prompt" "$baseline_instruction" "$assisted_instruction"; do
  [ -f "$input_path" ] && [ -r "$input_path" ] && [ -s "$input_path" ] ||
    die "input must be a non-empty readable regular file: $input_path"
done
prompt=$(canonical_file "$prompt") || die "cannot resolve prompt path"
baseline_instruction=$(canonical_file "$baseline_instruction") || die "cannot resolve baseline instruction path"
assisted_instruction=$(canonical_file "$assisted_instruction") || die "cannot resolve assisted instruction path"

[ ! -e "$output" ] || die "output path already exists: $output"
output_parent=$(canonical_directory "$(dirname -- "$output")") ||
  die "output parent directory does not exist"
output="$output_parent/$(basename -- "$output")"
case "$output/" in
  "$workspace/"*) die "output directory must be outside the benchmark workspace" ;;
esac

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-agent-benchmark.XXXXXX")
cleanup() {
  status=$?
  trap - EXIT
  rm -rf -- "$temporary_directory"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

: "${CODEX_BENCHMARK_ARGS:?CODEX_BENCHMARK_ARGS must contain one literal argument per line}"
args_source="$temporary_directory/codex-args.txt"
printf '%s\n' "$CODEX_BENCHMARK_ARGS" >"$args_source"
codex_args=()
while IFS= read -r argument || [ -n "$argument" ]; do
  [ -n "$argument" ] || die "CODEX_BENCHMARK_ARGS must not contain empty arguments"
  codex_args[${#codex_args[@]}]=$argument
done <"$args_source"

approval_count=0
sandbox_count=0
exec_count=0
model_count=0
reasoning_count=0
color_count=0
skip_git_count=0
ephemeral_count=0
ignore_rules_count=0
ignore_user_config_count=0

index=0
while [ "$index" -lt "${#codex_args[@]}" ]; do
  argument=${codex_args[$index]}
  case "$argument" in
    exec)
      exec_count=$((exec_count + 1))
      ;;
    -a|--ask-for-approval)
      index=$((index + 1))
      [ "$index" -lt "${#codex_args[@]}" ] || die "$argument requires a value"
      [ "${codex_args[$index]}" = "never" ] || die "approval mode must be never"
      approval_count=$((approval_count + 1))
      ;;
    --ask-for-approval=never)
      approval_count=$((approval_count + 1))
      ;;
    --ask-for-approval=*)
      die "approval mode must be never"
      ;;
    -s|--sandbox)
      index=$((index + 1))
      [ "$index" -lt "${#codex_args[@]}" ] || die "$argument requires a value"
      [ "${codex_args[$index]}" = "read-only" ] || die "sandbox must be read-only"
      sandbox_count=$((sandbox_count + 1))
      ;;
    --sandbox=read-only)
      sandbox_count=$((sandbox_count + 1))
      ;;
    --sandbox=*)
      die "sandbox must be read-only"
      ;;
    -m|--model)
      index=$((index + 1))
      [ "$index" -lt "${#codex_args[@]}" ] || die "$argument requires a value"
      require_nonblank_model "${codex_args[$index]}"
      model_count=$((model_count + 1))
      ;;
    --model=*)
      require_nonblank_model "${argument#--model=}"
      model_count=$((model_count + 1))
      ;;
    -c|--config)
      index=$((index + 1))
      [ "$index" -lt "${#codex_args[@]}" ] || die "$argument requires a value"
      is_reasoning_config "${codex_args[$index]}" ||
        die "unsupported Codex config override: ${codex_args[$index]}"
      reasoning_count=$((reasoning_count + 1))
      ;;
    --config=*)
      is_reasoning_config "${argument#--config=}" ||
        die "unsupported Codex config override: ${argument#--config=}"
      reasoning_count=$((reasoning_count + 1))
      ;;
    --color)
      index=$((index + 1))
      [ "$index" -lt "${#codex_args[@]}" ] || die "--color requires a value"
      [ "${codex_args[$index]}" = "never" ] || die "color mode must be never"
      color_count=$((color_count + 1))
      ;;
    --color=never)
      color_count=$((color_count + 1))
      ;;
    --color=*)
      die "color mode must be never"
      ;;
    --skip-git-repo-check) skip_git_count=$((skip_git_count + 1)) ;;
    --ephemeral) ephemeral_count=$((ephemeral_count + 1)) ;;
    --ignore-rules) ignore_rules_count=$((ignore_rules_count + 1)) ;;
    --ignore-user-config) ignore_user_config_count=$((ignore_user_config_count + 1)) ;;
    --dangerously-bypass-approvals-and-sandbox|--dangerously-bypass-hook-trust|--search|--json|--add-dir|--cd|-C)
      die "unsafe or script-controlled Codex argument: $argument"
      ;;
    --dangerously-bypass-approvals-and-sandbox=*|--dangerously-bypass-hook-trust=*|--search=*|--json=*|--add-dir=*|--cd=*|-C*)
      die "unsafe or script-controlled Codex argument: $argument"
      ;;
    -*)
      die "unsupported Codex benchmark argument: $argument"
      ;;
    *)
      die "unexpected positional Codex argument: $argument"
      ;;
  esac
  index=$((index + 1))
done

[ "$approval_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must set approval never exactly once"
[ "$sandbox_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must set sandbox read-only exactly once"
[ "$exec_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must contain exactly one exec argument"
[ "$model_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must set one explicit model"
[ "$reasoning_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must set model_reasoning_effort exactly once"
[ "$color_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must set color never exactly once"
[ "$skip_git_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must contain --skip-git-repo-check exactly once"
[ "$ephemeral_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must contain --ephemeral exactly once"
[ "$ignore_rules_count" -eq 1 ] || die "CODEX_BENCHMARK_ARGS must contain --ignore-rules exactly once"
[ "$ignore_user_config_count" -eq 1 ] ||
  die "CODEX_BENCHMARK_ARGS must contain --ignore-user-config exactly once"

mkdir -- "$output"
mkdir -- "$output/inputs"
cp -- "$prompt" "$output/inputs/base-prompt.txt"
cp -- "$baseline_instruction" "$output/inputs/baseline-instruction.txt"
cp -- "$assisted_instruction" "$output/inputs/assisted-instruction.txt"

if grep -Eiq 'goregraph|goregraph-out|\.goregraph-workspace|task_context|context[[:space:]]+pack' \
  "$output/inputs/base-prompt.txt"; then
  die "base prompt is not GoreGraph-neutral"
fi

expected_baseline='Do not use the goregraph CLI, MCP tools, goregraph-out, or .goregraph-workspace files.'
expected_assisted='Call goregraph context once with the complete task before reading indexed source.
Treat source_sections as current source already read; do not re-read or grep included ranges.
If source_coverage is complete, continue from the included source without another navigation read.
If source_coverage is partial or none, inspect only exact project/path entries listed in source_omissions; report pathless omissions as uncertainty without broad source discovery.
Never inventory repositories or read or grep outside included source_section ranges to reconstruct their files.
If fallback_required is true, confidence is low, or there is not exactly one reliable production entrypoint, stop using GoreGraph.
Retry only when retry_allowed is true: call once with exactly one retry_anchor and --previous-context-id <context_id>; never repeat or expand the original task.
Do not use specialist GoreGraph queries or expert MCP tools.'
expected_baseline_file="$temporary_directory/expected-baseline.txt"
expected_baseline_no_newline="$temporary_directory/expected-baseline-no-newline.txt"
expected_assisted_file="$temporary_directory/expected-assisted.txt"
expected_assisted_no_newline="$temporary_directory/expected-assisted-no-newline.txt"
printf '%s\n' "$expected_baseline" >"$expected_baseline_file"
printf '%s' "$expected_baseline" >"$expected_baseline_no_newline"
printf '%s\n' "$expected_assisted" >"$expected_assisted_file"
printf '%s' "$expected_assisted" >"$expected_assisted_no_newline"
if ! cmp -s "$output/inputs/baseline-instruction.txt" "$expected_baseline_file" &&
  ! cmp -s "$output/inputs/baseline-instruction.txt" "$expected_baseline_no_newline"; then
  die "baseline instruction does not match docs/BENCHMARKING.md"
fi
if ! cmp -s "$output/inputs/assisted-instruction.txt" "$expected_assisted_file" &&
  ! cmp -s "$output/inputs/assisted-instruction.txt" "$expected_assisted_no_newline"; then
  die "assisted instruction does not match docs/BENCHMARKING.md"
fi

{
  cat "$output/inputs/base-prompt.txt"
  printf '\n'
  cat "$expected_baseline_file"
} >"$output/baseline-prompt.txt"
{
  cat "$output/inputs/base-prompt.txt"
  printf '\n'
  cat "$expected_assisted_file"
} >"$output/assisted-prompt.txt"

: >"$output/codex-args.txt"
for argument in "${codex_args[@]}"; do
  printf '%s\n' "$argument" >>"$output/codex-args.txt"
done
codex --version >"$output/codex-version.txt" 2>&1
goregraph version >"$output/goregraph-version.txt" 2>&1
goregraph context "$workspace" --query "benchmark context preflight" --format json \
  >"$output/context-preflight.json"

printf 'variant\trun\ttokens\ttool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tincluded_source_rereads\tunique_source_files\tlog\n' >"$output/summary.tsv"
baseline_tokens="$temporary_directory/baseline.tokens"
assisted_tokens="$temporary_directory/assisted.tokens"
baseline_tool_calls="$temporary_directory/baseline.tool-calls"
assisted_tool_calls="$temporary_directory/assisted.tool-calls"
baseline_navigation_calls="$temporary_directory/baseline.navigation-calls"
assisted_navigation_calls="$temporary_directory/assisted.navigation-calls"
baseline_source_read_calls="$temporary_directory/baseline.source-read-calls"
assisted_source_read_calls="$temporary_directory/assisted.source-read-calls"
assisted_repeated_full_packs="$temporary_directory/assisted.repeated-full-packs"
assisted_included_source_rereads="$temporary_directory/assisted.included-source-rereads"
: >"$baseline_tokens"
: >"$assisted_tokens"
: >"$baseline_tool_calls"
: >"$assisted_tool_calls"
: >"$baseline_navigation_calls"
: >"$assisted_navigation_calls"
: >"$baseline_source_read_calls"
: >"$assisted_source_read_calls"
: >"$assisted_repeated_full_packs"
: >"$assisted_included_source_rereads"

extract_tokens() {
  go run "$analyzer_go" --tokens "$1"
}

run_variant() {
  variant=$1
  run_number=$2
  prompt_path="$output/$variant-prompt.txt"
  log_path="$output/$variant-$run_number.log"
  stderr_path="$log_path.stderr"
  metrics_path="$log_path.metrics.tsv"

  set +e
  codex "${codex_args[@]}" --json -C "$workspace" - <"$prompt_path" >"$log_path" 2>"$stderr_path"
  codex_status=$?
  set -e
  [ "$codex_status" -eq 0 ] ||
    die "$variant run $run_number failed with exit $codex_status; JSONL log retained at $log_path; stderr retained at $stderr_path"

  tokens=$(extract_tokens "$log_path")
  [ -n "$tokens" ] || die "cannot extract tokens from $log_path"
  metrics=$(bash "$analyzer" "$log_path") ||
    die "cannot analyze transcript: $log_path"
  printf '%s\n' "$metrics" >"$metrics_path"
  IFS=$'\t' read -r tool_calls goregraph_calls full_context_packs compact_duplicate_packs \
    repeated_full_packs raw_navigation_calls source_read_calls included_source_rereads \
    unique_source_files extra_metrics <<EOF
$metrics
EOF
  [ -z "${extra_metrics:-}" ] || die "invalid analyzer result: $metrics_path"
  for metric in "$tool_calls" "$goregraph_calls" "$full_context_packs" \
    "$compact_duplicate_packs" "$repeated_full_packs" "$raw_navigation_calls" \
    "$source_read_calls" "$included_source_rereads" "$unique_source_files"; do
    case "$metric" in
      *[!0-9]*|"") die "invalid analyzer result: $metrics_path" ;;
    esac
  done
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$variant" "$run_number" "$tokens" "$tool_calls" "$goregraph_calls" \
    "$full_context_packs" "$compact_duplicate_packs" "$repeated_full_packs" \
    "$raw_navigation_calls" "$source_read_calls" "$included_source_rereads" \
    "$unique_source_files" "$log_path" \
    >>"$output/summary.tsv"
  printf '%s\n' "$tokens" >>"$temporary_directory/$variant.tokens"
  printf '%s\n' "$tool_calls" >>"$temporary_directory/$variant.tool-calls"
  printf '%s\n' "$raw_navigation_calls" >>"$temporary_directory/$variant.navigation-calls"
  printf '%s\n' "$source_read_calls" >>"$temporary_directory/$variant.source-read-calls"
  if [ "$variant" = "assisted" ]; then
    printf '%s\n' "$repeated_full_packs" >>"$assisted_repeated_full_packs"
    printf '%s\n' "$included_source_rereads" >>"$assisted_included_source_rereads"
  fi
}

run_number=1
while [ "$run_number" -le "$runs" ]; do
  if [ $((run_number % 2)) -eq 1 ]; then
    run_variant baseline "$run_number"
    run_variant assisted "$run_number"
  else
    run_variant assisted "$run_number"
    run_variant baseline "$run_number"
  fi
  run_number=$((run_number + 1))
done

middle=$(((runs + 1) / 2))
median() {
  sort -n "$1" | sed -n "${middle}p"
}

baseline_median=$(median "$baseline_tokens")
assisted_median=$(median "$assisted_tokens")
baseline_tool_median=$(median "$baseline_tool_calls")
assisted_tool_median=$(median "$assisted_tool_calls")
baseline_navigation_median=$(median "$baseline_navigation_calls")
assisted_navigation_median=$(median "$assisted_navigation_calls")
baseline_source_read_median=$(median "$baseline_source_read_calls")
assisted_source_read_median=$(median "$assisted_source_read_calls")
[ -n "$baseline_median" ] && [ -n "$assisted_median" ] && \
  [ -n "$baseline_tool_median" ] && [ -n "$assisted_tool_median" ] && \
  [ -n "$baseline_navigation_median" ] && [ -n "$assisted_navigation_median" ] && \
  [ -n "$baseline_source_read_median" ] && [ -n "$assisted_source_read_median" ] ||
  die "cannot calculate benchmark medians"
assisted_repeated_full_packs=$(awk '{ total += $1 } END { print total + 0 }' "$assisted_repeated_full_packs")
assisted_included_source_rereads=$(awk '{ total += $1 } END { print total + 0 }' "$assisted_included_source_rereads")

printf 'baseline\tmedian\t%s\t%s\t-\t-\t-\t-\t%s\t%s\t-\t-\t-\n' \
  "$baseline_median" "$baseline_tool_median" "$baseline_navigation_median" \
  "$baseline_source_read_median" >>"$output/summary.tsv"
printf 'assisted\tmedian\t%s\t%s\t-\t-\t-\t-\t%s\t%s\t-\t-\t-\n' \
  "$assisted_median" "$assisted_tool_median" "$assisted_navigation_median" \
  "$assisted_source_read_median" >>"$output/summary.tsv"

printf 'Baseline median: %s tokens\n' "$baseline_median"
printf 'Assisted median: %s tokens\n' "$assisted_median"
printf 'Baseline/assisted tool-call medians: %s/%s\n' "$baseline_tool_median" "$assisted_tool_median"
printf 'Baseline/assisted raw-navigation medians: %s/%s\n' \
  "$baseline_navigation_median" "$assisted_navigation_median"
printf 'Baseline/assisted source-read medians: %s/%s\n' \
  "$baseline_source_read_median" "$assisted_source_read_median"
printf 'Complete the quality rubric in docs/BENCHMARKING.md before release.\n'

[ $((assisted_median * 5)) -le $((baseline_median * 4)) ] ||
  die "assisted median exceeds 80% of matched baseline"
[ "$assisted_median" -le 116560 ] ||
  die "assisted median exceeds the recorded-baseline gate of 116560"
[ "$baseline_source_read_median" -gt 0 ] ||
  die "baseline source-read median is zero; benchmark cannot measure source-replacement savings"
[ $((assisted_tool_median * 10)) -le $((baseline_tool_median * 7)) ] ||
  die "assisted tool-call median exceeds 70% of matched baseline"
[ $((assisted_source_read_median * 2)) -le "$baseline_source_read_median" ] ||
  die "assisted source-read median exceeds 50% of matched baseline"
[ "$assisted_repeated_full_packs" -eq 0 ] ||
  die "assisted runs repeated a full Context Pack"
[ "$assisted_included_source_rereads" -eq 0 ] ||
  die "assisted runs re-read source files already included by complete Context Packs"

printf 'Token and structural gates passed. Raw evidence: %s\n' "$output"
