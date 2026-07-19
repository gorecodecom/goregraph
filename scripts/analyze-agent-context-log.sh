#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

usage() {
  cat <<'EOF'
Usage: scripts/analyze-agent-context-log.sh [--header] /absolute/path/to/transcript.log

Print one tab-separated row of transcript structural metrics.
EOF
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 2
}

header=0
if [ "${1:-}" = "--header" ]; then
  header=1
  shift
fi
[ "$#" -eq 1 ] || {
  usage >&2
  exit 2
}

transcript=$1
[ -f "$transcript" ] && [ -r "$transcript" ] ||
  die "transcript must be a readable regular file: $transcript"

if [ "$header" -eq 1 ]; then
  printf 'tool_calls\tgoregraph_calls\tfull_context_packs\tduplicate_context_packs\traw_navigation_calls\tsource_read_calls\tunique_source_files\n'
  exit 0
fi

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/goregraph-context-log.XXXXXX")
cleanup() {
  status=$?
  trap - EXIT
  rm -rf -- "$temporary_directory"
  exit "$status"
}
trap cleanup EXIT

is_source_path() {
  case "$1" in
    *.asm|*.bash|*.c|*.cc|*.clj|*.cpp|*.cs|*.css|*.cxx|*.dart|*.elm|*.ex|*.exs|*.fs|*.fsi|*.go|*.groovy|*.gvy|*.h|*.hpp|*.hrl|*.hs|*.html|*.java|*.jl|*.js|*.jsx|*.kt|*.kts|*.lua|*.m|*.mjs|*.mm|*.php|*.pl|*.pm|*.py|*.r|*.rb|*.rs|*.scala|*.scss|*.sh|*.sol|*.sql|*.swift|*.ts|*.tsx|*.vue|*.zsh)
      return 0
      ;;
  esac
  return 1
}

trim_path_token() {
  value=$1
  value=${value#\"}
  value=${value#\'}
  value=${value#\(}
  value=${value%\"}
  value=${value%\'}
  value=${value%\)}
  value=${value%,}
  value=${value%;}
  value=${value%:}
  printf '%s\n' "$value"
}

record_source_paths() {
  value=$1
  found=1
  for token in $value; do
    path=$(trim_path_token "$token")
    if is_source_path "$path"; then
      printf '%s\n' "$path" >>"$temporary_directory/source-paths"
      found=0
    fi
  done
  return "$found"
}

json_string() {
  field=$1
  line=$2
  printf '%s\n' "$line" |
    sed -nE 's/.*"'"$field"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' |
    sed -n '1p'
}

tool_calls=0
goregraph_context_calls=0
task_context_calls=0
full_context_packs=0
duplicate_context_packs=0
raw_navigation_calls=0
source_read_calls=0
: >"$temporary_directory/context-ids"
: >"$temporary_directory/source-paths"

while IFS= read -r line || [ -n "$line" ]; do
  record_type=$(json_string type "$line")
  case "$record_type" in
    tool_call|tool_use|exec|function_call|command_execution|mcp_call|mcp_tool_call|read|read_file|open_file|view_file|task_context)
      is_tool_invocation=1
      ;;
    *)
      is_tool_invocation=0
      ;;
  esac
  if [ "$is_tool_invocation" -eq 1 ]; then
    tool_calls=$((tool_calls + 1))
    tool_name=$(json_string tool_name "$line")
    [ -n "$tool_name" ] || tool_name=$(json_string name "$line")
    [ -n "$tool_name" ] || tool_name=$(json_string tool "$line")
    [ -n "$tool_name" ] || tool_name=$record_type
    command=$(json_string command "$line")
    [ -n "$command" ] || command=$(json_string cmd "$line")
    path=$(json_string path "$line")

    if [ "$tool_name" = "task_context" ]; then
      task_context_calls=$((task_context_calls + 1))
    elif printf '%s\n' "$command" | grep -Eq '(^|[[:space:]])([^[:space:]]*/)?goregraph[[:space:]]+context([[:space:]]|$)'; then
      goregraph_context_calls=$((goregraph_context_calls + 1))
    else
      command_name=${command%%[[:space:]]*}
      case "$command_name" in
        rg|grep|find)
          if record_source_paths "$command"; then
            raw_navigation_calls=$((raw_navigation_calls + 1))
          fi
          ;;
        sed|nl|cat|head|tail)
          if record_source_paths "$command"; then
            raw_navigation_calls=$((raw_navigation_calls + 1))
            source_read_calls=$((source_read_calls + 1))
          fi
          ;;
        *)
          case "$tool_name" in
            read|read_file|open_file|view_file)
              if record_source_paths "$path"; then
                raw_navigation_calls=$((raw_navigation_calls + 1))
                source_read_calls=$((source_read_calls + 1))
              fi
              ;;
          esac
          ;;
      esac
    fi
  fi

  context_id=$(json_string context_id "$line")
  if [ -n "$context_id" ]; then
    duplicate_of=$(json_string duplicate_of "$line")
    if [ -n "$duplicate_of" ] || grep -Fqx -- "$context_id" "$temporary_directory/context-ids"; then
      duplicate_context_packs=$((duplicate_context_packs + 1))
    else
      full_context_packs=$((full_context_packs + 1))
      printf '%s\n' "$context_id" >>"$temporary_directory/context-ids"
    fi
  fi
done <"$transcript"

[ "$tool_calls" -gt 0 ] || die "transcript has no parseable tool invocations"

unique_source_files=0
if [ -s "$temporary_directory/source-paths" ]; then
  unique_source_files=$(sort -u "$temporary_directory/source-paths" | awk 'END { print NR }')
fi
goregraph_calls=$((goregraph_context_calls + task_context_calls))

printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
  "$tool_calls" "$goregraph_calls" "$full_context_packs" "$duplicate_context_packs" \
  "$raw_navigation_calls" "$source_read_calls" "$unique_source_files"
