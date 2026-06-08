#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$root"

fail() {
  echo "docs gate failed: $*" >&2
  exit 1
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || fail "missing $path"
  [[ -s "$path" ]] || fail "$path is empty"
}

require_file "design.md"
require_file "tasks.md"

grep -Eq '```[[:space:]]*mermaid' design.md ||
  fail "design.md must contain a Mermaid diagram fence"

grep -Eq '^[[:space:]]*(flowchart|graph)[[:space:]]+' design.md ||
  fail "design.md Mermaid diagram must include an architecture graph"

grep -Eq '^\|[[:space:]]*ID[[:space:]]*\|' design.md ||
  fail "design.md must contain a risk register table with an ID column"

grep -Eq '^\|[[:space:]]*R[0-9]+[[:space:]]*\|' design.md ||
  fail "design.md risk register table must contain at least one risk row"

grep -Eq '^\|[[:space:]]*Stage[[:space:]]*\|' tasks.md ||
  fail "tasks.md must contain a Stage status table"

grep -Eq 'Stage[[:space:]]+[0-9]+' tasks.md ||
  fail "tasks.md must list at least one numbered Stage"

grep -Eq 'In progress|Planned|Done|Complete|Completed|Pending|Active' tasks.md ||
  fail "tasks.md must include Stage status values"

echo "docs gate passed"
