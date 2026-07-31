#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
analyzer="$script_dir/analyze-agent-context-log.sh"

[ "$#" -eq 1 ] || {
  printf 'usage: %s /absolute/evidence/directory\n' "$0" >&2
  exit 2
}
evidence=$1
[ "${evidence#/}" != "$evidence" ] || {
  printf 'error: evidence directory must be absolute\n' >&2
  exit 2
}
[ -d "$evidence" ] || {
  printf 'error: evidence directory is missing: %s\n' "$evidence" >&2
  exit 2
}

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-token-calibration.XXXXXX")
cleanup() {
  status=$?
  trap - EXIT
  rm -rf -- "$temporary_directory"
  exit "$status"
}
trap cleanup EXIT

printf 'variant\trun\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\teffective_tokens\tlog\n'
for variant in baseline assisted; do
  values="$temporary_directory/$variant.effective"
  : >"$values"
  for run in 1 2 3; do
    log="$evidence/$variant-$run.log"
    [ -f "$log" ] && [ -r "$log" ] || {
      printf 'error: required transcript is missing: %s\n' "$log" >&2
      exit 2
    }
    usage=$(bash "$analyzer" --usage "$log")
    IFS=$'\t' read -r input cached uncached output reasoning total effective <<EOF
$usage
EOF
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$variant" "$run" "$input" "$cached" "$uncached" "$output" \
      "$reasoning" "$total" "$effective" "$log"
    printf '%s\n' "$effective" >>"$values"
  done
done

median() {
  sort -n "$1" | sed -n '2p'
}
baseline_median=$(median "$temporary_directory/baseline.effective")
assisted_median=$(median "$temporary_directory/assisted.effective")
ratio=$((assisted_median * 100 / baseline_median))
printf 'baseline\tmedian\t-\t-\t-\t-\t-\t-\t%s\t-\n' "$baseline_median"
printf 'assisted\tmedian\t-\t-\t-\t-\t-\t-\t%s\t-\n' "$assisted_median"
printf 'gate\tmatched_ratio_percent\t-\t-\t-\t-\t-\t-\t%s\t-\n' "$ratio"
printf 'gate\tabsolute_cap\t-\t-\t-\t-\t-\t-\t116560\t-\n'
