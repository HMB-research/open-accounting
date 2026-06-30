#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" || fail "not inside a git repository"
cd "$repo_root"

canonical_author="Martin Kask <martin@industrial.ninja>"
blocked_pattern='Claude|claude|anthropic|claude-code'

shortlog="$(git shortlog -sne --all)"
if printf '%s\n' "$shortlog" | grep -E "$blocked_pattern" >/dev/null; then
  printf '%s\n' "$shortlog" >&2
  fail "normalized contributor list still contains Claude or Anthropic identities"
fi

mapped_identities="$(git log --all --format='%aN <%aE>%n%cN <%cE>' | sort -u)"
if printf '%s\n' "$mapped_identities" | grep -E "$blocked_pattern" >/dev/null; then
  printf '%s\n' "$mapped_identities" >&2
  fail "mailmapped author or committer identities still contain Claude or Anthropic identities"
fi

aliases=(
  "Claude <noreply@anthropic.com>"
  "Claude <claude@anthropic.com>"
  "Claude Opus 4.5 <noreply@anthropic.com>"
  "<noreply@anthropic.com>"
  "<claude@anthropic.com>"
  "<158136808+claude-code@users.noreply.github.com>"
)

for alias in "${aliases[@]}"; do
  mapped="$(git check-mailmap "$alias")"
  if [ "$mapped" != "$canonical_author" ]; then
    fail "mailmap maps '$alias' to '$mapped', expected '$canonical_author'"
  fi
done

echo "Contributor attribution OK"
printf '%s\n' "$shortlog"
