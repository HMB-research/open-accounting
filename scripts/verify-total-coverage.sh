#!/bin/sh
set -eu

profile="${1:-coverage.out}"

if [ ! -f "$profile" ]; then
	echo "coverage profile not found: $profile" >&2
	exit 1
fi

awk '
	NR == 1 {
		next
	}
	{
		statements += $2
		if ($3 > 0) {
			covered += $2
		} else {
			uncovered = uncovered " " $1
		}
	}
	END {
		if (statements == 0) {
			print "coverage profile has no statements" > "/dev/stderr"
			exit 1
		}
		missed = statements - covered
		printf("backend coverage: statements=%d missed=%d covered=%d coverage=%.4f%%\n", statements, missed, covered, covered * 100 / statements)
		if (missed != 0) {
			print "uncovered coverage blocks:" uncovered > "/dev/stderr"
			exit 1
		}
	}
' "$profile"
