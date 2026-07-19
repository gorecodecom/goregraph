#!/usr/bin/env bash

set -euo pipefail
set -f
export LC_ALL=C

usage() {
  cat <<'EOF'
Usage: scripts/analyze-agent-context-log.sh [--header] /absolute/path/to/transcript.jsonl

Print one tab-separated row of terminal Codex JSONL tool metrics.
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
  printf 'tool_calls\tgoregraph_calls\tfull_context_packs\tcompact_duplicate_packs\trepeated_full_packs\traw_navigation_calls\tsource_read_calls\tunique_source_files\n'
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

json_string() {
  field=$1
  line=$2
  printf '%s\n' "$line" |
    sed -nE 's/.*"'"$field"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' |
    sed -n '1p'
}

item_string() {
  field=$1
  line=$2
  printf '%s\n' "$line" |
    sed -nE 's/.*"item"[[:space:]]*:[[:space:]]*\{.*"'"$field"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' |
    sed -n '1p'
}

is_json_object() {
  line=$1
  case "$line" in
    \{*\}) ;;
    *) return 1 ;;
  esac
  depth=0
  in_string=0
  escaped=0
  index=0
  while [ "$index" -lt "${#line}" ]; do
    character=${line:index:1}
    if [ "$in_string" -eq 1 ]; then
      if [ "$escaped" -eq 1 ]; then
        escaped=0
      elif [ "$character" = '\\' ]; then
        escaped=1
      elif [ "$character" = '"' ]; then
        in_string=0
      fi
    else
      case "$character" in
        '"') in_string=1 ;;
        '{') depth=$((depth + 1)) ;;
        '}')
          depth=$((depth - 1))
          [ "$depth" -ge 0 ] || return 1
          ;;
      esac
    fi
    index=$((index + 1))
  done
  [ "$in_string" -eq 0 ] && [ "$depth" -eq 0 ]
}

is_source_path() {
  case "$1" in
    *.asm|*.bash|*.c|*.cc|*.clj|*.cpp|*.cs|*.css|*.cxx|*.dart|*.elm|*.ex|*.exs|*.fs|*.fsi|*.go|*.groovy|*.gvy|*.h|*.hpp|*.hrl|*.hs|*.html|*.java|*.jl|*.js|*.jsx|*.kt|*.kts|*.lua|*.m|*.mjs|*.mm|*.php|*.pl|*.pm|*.py|*.r|*.rb|*.rs|*.scala|*.scss|*.sh|*.sol|*.sql|*.swift|*.ts|*.tsx|*.vue|*.zig|*.zsh)
      return 0
      ;;
  esac
  return 1
}

normalize_path() {
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
  case "$value" in
    *'*'*|*'?'*|*'['*) return 1 ;;
  esac
  while [ "${value#./}" != "$value" ]; do
    value=${value#./}
  done
  [ -n "$value" ] || return 1
  printf '%s\n' "$value"
}

record_source_path() {
  value=$(normalize_path "$1") || return 1
  is_source_path "$value" || return 1
  printf '%s\n' "$value" >>"$temporary_directory/source-paths"
}

unwrap_command() {
  command=$1
  case "$command" in
    /bin/sh\ -lc\ *|/bin/bash\ -lc\ *|/bin/zsh\ -lc\ *|sh\ -lc\ *|bash\ -lc\ *|zsh\ -lc\ *)
      command=${command#* -lc }
      first=${command%"${command#?}"}
      last=${command#"${command%?}"}
      if { [ "$first" = "'" ] || [ "$first" = '"' ]; } && [ "$last" = "$first" ]; then
        command=${command#?}
        command=${command%?}
      fi
      ;;
  esac
  printf '%s\n' "$command"
}

record_search_targets() {
  command=$1
  shift
  pattern_seen=0
  option_value=0
  option_is_pattern=0
  end_options=0
  found=1
  for token in "$@"; do
    if [ "$option_value" -eq 1 ]; then
      if [ "$option_is_pattern" -eq 1 ]; then
        pattern_seen=1
      fi
      option_value=0
      option_is_pattern=0
      continue
    fi
    if [ "$end_options" -eq 0 ]; then
      case "$token" in
        --) end_options=1; continue ;;
        -e|--regexp|-f|--file)
          option_value=1
          option_is_pattern=1
          continue
          ;;
        -g|--glob|--type|--type-not) option_value=1; continue ;;
        --regexp=*|--file=*|--glob=*|--type=*|--type-not=*) continue ;;
        -*) continue ;;
      esac
    fi
    if [ "$pattern_seen" -eq 0 ]; then
      pattern_seen=1
      continue
    fi
    if record_source_path "$token"; then
      found=0
    fi
  done
  return "$found"
}

record_find_targets() {
  found=1
  for token in "$@"; do
    case "$token" in
      -name|-iname|-path|-ipath|-type|-exec|-execdir|-ok|-okdir|-print|-print0|-delete|-quit)
        break
        ;;
    esac
    if record_source_path "$token"; then
      found=0
    fi
  done
  return "$found"
}

record_read_targets() {
  command=$1
  shift
  script_seen=0
  option_value=0
  end_options=0
  found=1
  case "$command" in
    sed) script_required=1 ;;
    *) script_required=0 ;;
  esac
  for token in "$@"; do
    if [ "$option_value" -eq 1 ]; then
      option_value=0
      if [ "$command" = "sed" ]; then
        script_seen=1
      fi
      continue
    fi
    if [ "$end_options" -eq 0 ]; then
      case "$token" in
        --) end_options=1; continue ;;
      esac
      case "$command:$token" in
        sed:-e|sed:-f|nl:-b|nl:-d|nl:-f|nl:-h|nl:-i|nl:-l|nl:-n|nl:-p|nl:-s|nl:-v|nl:-w|head:-n|head:-c|tail:-n|tail:-c)
          option_value=1
          continue
          ;;
      esac
      case "$token" in
        -*) continue ;;
      esac
    fi
    if [ "$script_required" -eq 1 ] && [ "$script_seen" -eq 0 ]; then
      script_seen=1
      continue
    fi
    if record_source_path "$token"; then
      found=0
    fi
  done
  return "$found"
}

is_tool_item_type() {
  case "$1" in
    command_execution|mcp_tool_call|web_search|image_generation|computer_call|file_read|file_write|file_change|apply_patch)
      return 0
      ;;
  esac
  return 1
}

record_context_pack() {
  line=$1
  decoded=$(printf '%s\n' "$line" | sed 's/\\\\"/"/g; s/\\"/"/g')
  context_id=$(json_string context_id "$decoded")
  [ -n "$context_id" ] ||
    context_id=$(printf '%s\n' "$decoded" |
      sed -nE 's/.*context_id:[[:space:]]*`?([^[:space:]`"]+).*/\1/p' | sed -n '1p')
  [ -n "$context_id" ] || return 0
  duplicate_of=$(json_string duplicate_of "$decoded")
  [ -n "$duplicate_of" ] ||
    duplicate_of=$(printf '%s\n' "$decoded" |
      sed -nE 's/.*duplicate_of:[[:space:]]*`?([^[:space:]`"]+).*/\1/p' | sed -n '1p')
  if [ -n "$duplicate_of" ]; then
    compact_duplicate_packs=$((compact_duplicate_packs + 1))
  elif grep -Fqx -- "$context_id" "$temporary_directory/full-context-ids"; then
    repeated_full_packs=$((repeated_full_packs + 1))
  else
    full_context_packs=$((full_context_packs + 1))
    printf '%s\n' "$context_id" >>"$temporary_directory/full-context-ids"
  fi
}

tool_calls=0
goregraph_calls=0
full_context_packs=0
compact_duplicate_packs=0
repeated_full_packs=0
raw_navigation_calls=0
source_read_calls=0
: >"$temporary_directory/completed-item-ids"
: >"$temporary_directory/full-context-ids"
: >"$temporary_directory/source-paths"

while IFS= read -r line || [ -n "$line" ]; do
  [ -z "$line" ] && continue
  is_json_object "$line" || die "transcript contains invalid JSONL"
  printf '%s\n' "$line" | grep -Eq '"type"[[:space:]]*:[[:space:]]*"item\.completed"' || continue
  item_id=$(item_string id "$line")
  item_type=$(item_string type "$line")
  [ -n "$item_id" ] && [ -n "$item_type" ] || continue
  grep -Fqx -- "$item_id" "$temporary_directory/completed-item-ids" && continue
  printf '%s\n' "$item_id" >>"$temporary_directory/completed-item-ids"
  is_tool_item_type "$item_type" || continue

  tool_calls=$((tool_calls + 1))
  command=$(item_string command "$line")
  tool_name=$(item_string tool "$line")
  path=$(item_string path "$line")
  context_call=0
  if [ "$item_type" = "mcp_tool_call" ] && [ "$tool_name" = "task_context" ]; then
    context_call=1
  fi
  if [ "$item_type" = "command_execution" ]; then
    command=$(unwrap_command "$command")
    command_name=${command%%[[:space:]]*}
    case "$command_name" in
      goregraph)
        case " $command " in
          *' goregraph context '*) context_call=1 ;;
        esac
        ;;
      rg|grep)
        if record_search_targets "$command_name" ${command#"$command_name"}; then
          raw_navigation_calls=$((raw_navigation_calls + 1))
        fi
        ;;
      find)
        if record_find_targets ${command#"$command_name"}; then
          raw_navigation_calls=$((raw_navigation_calls + 1))
        fi
        ;;
      sed|nl|cat|head|tail)
        if record_read_targets "$command_name" ${command#"$command_name"}; then
          raw_navigation_calls=$((raw_navigation_calls + 1))
          source_read_calls=$((source_read_calls + 1))
        fi
        ;;
    esac
  elif [ "$item_type" = "file_read" ]; then
    if record_source_path "$path"; then
      raw_navigation_calls=$((raw_navigation_calls + 1))
      source_read_calls=$((source_read_calls + 1))
    fi
  fi
  if [ "$context_call" -eq 1 ]; then
    goregraph_calls=$((goregraph_calls + 1))
    record_context_pack "$line"
  fi
done <"$transcript"

[ "$tool_calls" -gt 0 ] || die "transcript has no parseable terminal tool items"

unique_source_files=0
if [ -s "$temporary_directory/source-paths" ]; then
  unique_source_files=$(sort -u "$temporary_directory/source-paths" | awk 'END { print NR }')
fi

printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
  "$tool_calls" "$goregraph_calls" "$full_context_packs" "$compact_duplicate_packs" \
  "$repeated_full_packs" "$raw_navigation_calls" "$source_read_calls" "$unique_source_files"
