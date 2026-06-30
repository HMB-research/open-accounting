#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" || fail "not inside a git repository"
cd "$repo_root"

ref="${1:-HEAD}"
blocked_pattern='Claude|claude|anthropic|claude-code'

shortlog="$(git shortlog -sne "$ref")"
if printf '%s\n' "$shortlog" | grep -E "$blocked_pattern" >/dev/null; then
  printf '%s\n' "$shortlog" >&2
  fail "contributor list still contains blocked generated identities"
fi

raw_identities="$(git log --no-use-mailmap "$ref" --format='%aN <%aE>%n%cN <%cE>' | sort -u)"
if printf '%s\n' "$raw_identities" | grep -E "$blocked_pattern" >/dev/null; then
  printf '%s\n' "$raw_identities" >&2
  fail "raw author or committer identities still contain blocked generated identities"
fi

message_matches="$(git log "$ref" --format='%h %s' --regexp-ignore-case --grep='Co-authored-by:.*\(Claude\|anthropic\|claude-code\)' --grep='Generated with \[Claude Code\]' || true)"
if [ -n "$message_matches" ]; then
  printf '%s\n' "$message_matches" >&2
  fail "commit messages still contain blocked generated attribution trailers"
fi

echo "Contributor attribution OK"
printf '%s\n' "$shortlog"
