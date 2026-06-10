#!/bin/sh
set -eu

profile="${1:-coverage.out}"
package_prefix="${CLI_COVERAGE_PACKAGE_PREFIX:-github.com/HMB-research/open-accounting/cmd/oa/}"

if [ ! -f "$profile" ]; then
	echo "coverage profile not found: $profile" >&2
	exit 1
fi

go tool cover -func="$profile" | awk -v prefix="$package_prefix" '
	index($1, prefix) == 1 {
		seen = 1
		pct = $3
		gsub(/%/, "", pct)
		if (pct + 0 < 100) {
			print
			failed = 1
		}
	}
	END {
		if (!seen) {
			printf("no cmd/oa coverage entries found in profile for prefix %s\n", prefix) > "/dev/stderr"
			exit 1
		}
		if (failed) {
			exit 1
		}
	}
'

echo "cmd/oa coverage is 100%"
