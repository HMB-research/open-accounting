#!/bin/sh
set -eu

case "${1:-}" in
	-h|--help)
		cat <<'USAGE'
Usage: gh run view <run-id> --job <job-id> --log | scripts/parse-go-test-package-times.sh

Reads Go test output from stdin and prints package duration weights as:

  ./package/path<TAB>seconds

GitHub Actions job prefixes are allowed. Set GO_TEST_MODULE_PREFIX to parse a
different module path.
USAGE
		exit 0
		;;
	"")
		;;
	*)
		echo "unexpected argument: $1" >&2
		exit 2
		;;
esac

module_prefix="${GO_TEST_MODULE_PREFIX:-github.com/HMB-research/open-accounting}"

awk -v prefix="$module_prefix" '
	function emit(package_field, duration_field) {
		seconds = duration_field
		sub(/s$/, "", seconds)

		if (index(package_field, prefix "/") == 1) {
			print "./" substr(package_field, length(prefix) + 2) "\t" seconds
			return
		}
		if (index(package_field, "./") == 1) {
			print package_field "\t" seconds
		}
	}

	{
		for (field = 1; field <= NF - 2; field++) {
			if ($field == "ok" && $(field + 2) ~ /^[0-9][0-9]*([.][0-9][0-9]*)?s$/) {
				emit($(field + 1), $(field + 2))
				next
			}
		}
	}
' | sort -k1,1 | awk '!seen[$1]++'
