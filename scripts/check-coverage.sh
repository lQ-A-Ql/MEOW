#!/usr/bin/env bash
set -euo pipefail

coverage_file="${1:-coverage.out}"
baseline="${COVERAGE_BASELINE:-35.0}"

fail() {
  echo "coverage gate failed: $*" >&2
  exit 1
}

[[ -f "$coverage_file" ]] || fail "missing coverage profile: $coverage_file"
[[ -s "$coverage_file" ]] || fail "coverage profile is empty: $coverage_file"

total="$(
  go tool cover -func="$coverage_file" |
    awk '/^total:/ { value=$NF; sub(/%$/, "", value); print value }'
)"

[[ -n "$total" ]] || fail "could not read total coverage from $coverage_file"

awk -v value="$total" 'BEGIN { exit(value ~ /^[0-9]+([.][0-9]+)?$/ ? 0 : 1) }' ||
  fail "invalid total coverage value: $total"

awk -v value="$baseline" 'BEGIN { exit(value ~ /^[0-9]+([.][0-9]+)?$/ ? 0 : 1) }' ||
  fail "invalid coverage baseline: $baseline"

echo "coverage total: ${total}%"
echo "coverage baseline: ${baseline}%"

if ! awk -v actual="$total" -v required="$baseline" 'BEGIN { exit((actual + 0) >= (required + 0) ? 0 : 1) }'; then
  fail "total coverage ${total}% is below baseline ${baseline}%"
fi

echo "coverage gate passed"
