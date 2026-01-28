#!/bin/bash
# Auto-fix common golangci-lint issues
# Usage: ./scripts/fix-lint.sh [--check-only]
set -e
cd "$(dirname "$0")/.."

CHECK_ONLY=false
if [[ "$1" == "--check-only" ]]; then
  CHECK_ONLY=true
fi

echo "🔧 Running gofmt..."
if $CHECK_ONLY; then
  UNFORMATTED=$(gofmt -l . 2>/dev/null | grep -v vendor || true)
  if [[ -n "$UNFORMATTED" ]]; then
    echo "❌ Unformatted files:"
    echo "$UNFORMATTED"
    exit 1
  fi
else
  gofmt -w .
  echo "✅ gofmt applied"
fi

echo "🔍 Running golangci-lint..."
if golangci-lint run --timeout=5m 2>&1; then
  echo "✅ All lint checks passed!"
  exit 0
else
  if $CHECK_ONLY; then
    echo "❌ Lint issues found. Run ./scripts/fix-lint.sh to auto-fix formatting."
    exit 1
  else
    echo "⚠️  Some lint issues require manual fixes (errcheck, staticcheck)."
    echo "   Run: golangci-lint run --timeout=5m"
    exit 1
  fi
fi
