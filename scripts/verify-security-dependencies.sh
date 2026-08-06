#!/usr/bin/env bash

set -euo pipefail

# Keep the minimum patched versions for dependencies with security advisories in
# the build gate. A future downgrade should fail before it reaches main even if
# Dependabot has not opened a new pull request yet.
minimum_versions=(
  "github.com/go-chi/chi/v5 v5.2.4"
  "github.com/jackc/pgx/v5 v5.9.2"
)

failed=0

for requirement in "${minimum_versions[@]}"; do
  read -r module minimum <<< "$requirement"
  actual="$(go list -m -f '{{.Version}}' "$module")"

  if [[ -z "$actual" ]]; then
    printf 'status=fail module=%s reason=missing-version\n' "$module" >&2
    failed=1
    continue
  fi

  lowest="$(printf '%s\n' "$minimum" "$actual" | sort -V | head -n 1)"
  if [[ "$lowest" != "$minimum" ]]; then
    printf 'status=fail module=%s actual=%s minimum=%s\n' "$module" "$actual" "$minimum" >&2
    failed=1
    continue
  fi

  printf 'status=pass module=%s version=%s minimum=%s\n' "$module" "$actual" "$minimum"
done

if (( failed != 0 )); then
  exit 1
fi
