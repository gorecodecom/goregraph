#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -P -- "$(dirname -- "$0")" && pwd -P)
calibrator="$script_dir/calibrate-agent-context-tokens.sh"
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-token-calibration-test.XXXXXX")
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

write_log() {
  path=$1
  input=$2
  output=$3
  cached=${4:-0}
  printf '%s\n' \
    '{"type":"item.completed","item":{"id":"terminal-tool","type":"web_search","query":"token calibration"}}' \
    "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":$input,\"cached_input_tokens\":$cached,\"output_tokens\":$output,\"reasoning_output_tokens\":0}}" \
    >"$path"
}

evidence="$temporary_directory/evidence"
mkdir -p "$evidence"
write_log "$evidence/baseline-1.log" 90000 10000
write_log "$evidence/baseline-2.log" 100000 10000
write_log "$evidence/baseline-3.log" 110000 10000
write_log "$evidence/assisted-1.log" 60000 0
write_log "$evidence/assisted-2.log" 70000 0
write_log "$evidence/assisted-3.log" 80000 0

expected_header=$'variant\trun\tinput_tokens\tcached_input_tokens\tuncached_input_tokens\toutput_tokens\treasoning_output_tokens\ttotal_tokens\teffective_tokens\tlog'
expected_footer=$'baseline\tmedian\t-\t-\t-\t-\t-\t-\t110000\t-\nassisted\tmedian\t-\t-\t-\t-\t-\t-\t70000\t-\ngate\tmatched_ratio_percent\t-\t-\t-\t-\t-\t-\t63\t-\ngate\tabsolute_cap\t-\t-\t-\t-\t-\t-\t116560\t-'

bash "$calibrator" "$evidence" >"$temporary_directory/first.tsv"
header=$(sed -n '1p' "$temporary_directory/first.tsv")
[ "$header" = "$expected_header" ] || fail "header = $header"
footer=$(tail -n 4 "$temporary_directory/first.tsv")
[ "$footer" = "$expected_footer" ] || fail "footer = $footer"

bash "$calibrator" "$evidence" >"$temporary_directory/second.tsv"
sed "s|$temporary_directory|<temporary>|g" "$temporary_directory/first.tsv" >"$temporary_directory/first-normalized.tsv"
sed "s|$temporary_directory|<temporary>|g" "$temporary_directory/second.tsv" >"$temporary_directory/second-normalized.tsv"
cmp -s "$temporary_directory/first-normalized.tsv" "$temporary_directory/second-normalized.tsv" ||
  fail 'calibration output is not deterministic'
[ "$(find "$evidence" -type f | wc -l | tr -d ' ')" = '6' ] ||
  fail 'calibration created a file in the evidence directory'

missing="$temporary_directory/missing"
mkdir -p "$missing"
for variant in baseline assisted; do
  for run in 1 2 3; do
    [ "$variant-$run" = 'assisted-3' ] && continue
    write_log "$missing/$variant-$run.log" 100 0
  done
done
if bash "$calibrator" "$missing" >"$temporary_directory/missing.stdout" 2>"$temporary_directory/missing.stderr"; then
  fail 'missing transcript passed'
fi
grep -q 'required transcript is missing' "$temporary_directory/missing.stderr" ||
  fail 'missing transcript error was not specific'

malformed="$temporary_directory/malformed"
mkdir -p "$malformed"
for variant in baseline assisted; do
  for run in 1 2 3; do
    write_log "$malformed/$variant-$run.log" 100 0
  done
done
write_log "$malformed/baseline-1.log" 10 1 11
if bash "$calibrator" "$malformed" >"$temporary_directory/malformed.stdout" 2>"$temporary_directory/malformed.stderr"; then
  fail 'malformed usage passed'
fi
grep -q 'cached_input_tokens exceeds input_tokens' "$temporary_directory/malformed.stderr" ||
  fail 'malformed usage error was not specific'

printf 'PASS: calibrate-agent-context-tokens\n'
