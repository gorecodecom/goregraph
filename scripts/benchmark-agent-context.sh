#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

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

for command_name in codex goregraph awk sed sort grep mktemp cat cp cmp mkdir rm dirname basename; do
  require_command "$command_name"
done

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
expected_assisted='Call goregraph context once with the task and its default budget.
Read only cited source needed for verification.
If fallback_required is true, stop using GoreGraph.
At most one narrower exact retry is allowed.'
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

printf 'variant\trun\ttokens\tlog\n' >"$output/summary.tsv"
baseline_tokens="$temporary_directory/baseline.tokens"
assisted_tokens="$temporary_directory/assisted.tokens"
: >"$baseline_tokens"
: >"$assisted_tokens"

extract_tokens() {
  awk '
    function normalize_grouped(value, separator, count, parts, part, result) {
      if (value ~ /^[0-9]+$/) {
        return value
      }
      if (index(value, ".") > 0 && index(value, ",") > 0) {
        return ""
      }
      separator = index(value, ".") > 0 ? "\\." : ","
      count = split(value, parts, separator)
      if (count < 2 || length(parts[1]) < 1 || length(parts[1]) > 3 ||
          parts[1] !~ /^[0-9]+$/) {
        return ""
      }
      result = parts[1]
      for (part = 2; part <= count; part++) {
        if (length(parts[part]) != 3 || parts[part] !~ /^[0-9]+$/) {
          return ""
        }
        result = result parts[part]
      }
      return result
    }
    BEGIN { waiting = 0; candidate = "" }
    tolower($0) ~ /^[[:space:]]*tokens used[[:space:]]*$/ {
      waiting = 1
      next
    }
    waiting {
      value = $0
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      if (value ~ /^[0-9][0-9.,]*$/) {
        normalized = normalize_grouped(value)
        if (normalized ~ /^[0-9]+$/ && normalized + 0 > 0) {
          candidate = normalized
        }
      }
      if (value != "") {
        waiting = 0
      }
    }
    END {
      if (candidate != "") {
        print candidate
      }
    }
  ' "$1"
}

run_variant() {
  variant=$1
  run_number=$2
  prompt_path="$output/$variant-prompt.txt"
  log_path="$output/$variant-$run_number.log"

  set +e
  codex "${codex_args[@]}" -C "$workspace" - <"$prompt_path" >"$log_path" 2>&1
  codex_status=$?
  set -e
  [ "$codex_status" -eq 0 ] ||
    die "$variant run $run_number failed with exit $codex_status; log retained at $log_path"

  tokens=$(extract_tokens "$log_path")
  [ -n "$tokens" ] || die "cannot extract tokens from $log_path"
  printf '%s\t%s\t%s\t%s\n' "$variant" "$run_number" "$tokens" "$log_path" \
    >>"$output/summary.tsv"
  printf '%s\n' "$tokens" >>"$temporary_directory/$variant.tokens"
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
baseline_median=$(sort -n "$baseline_tokens" | sed -n "${middle}p")
assisted_median=$(sort -n "$assisted_tokens" | sed -n "${middle}p")
[ -n "$baseline_median" ] && [ -n "$assisted_median" ] ||
  die "cannot calculate benchmark medians"

printf 'baseline\tmedian\t%s\t-\n' "$baseline_median" >>"$output/summary.tsv"
printf 'assisted\tmedian\t%s\t-\n' "$assisted_median" >>"$output/summary.tsv"

printf 'Baseline median: %s tokens\n' "$baseline_median"
printf 'Assisted median: %s tokens\n' "$assisted_median"
printf 'Complete the quality rubric in docs/BENCHMARKING.md before release.\n'

[ $((assisted_median * 5)) -le $((baseline_median * 4)) ] ||
  die "assisted median exceeds 80% of matched baseline"
[ "$assisted_median" -le 116560 ] ||
  die "assisted median exceeds the recorded-baseline gate of 116560"

printf 'Token gates passed. Raw evidence: %s\n' "$output"
