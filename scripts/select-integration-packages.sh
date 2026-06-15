#!/bin/sh
set -eu

shard="${INTEGRATION_SHARD:-}"
shards="${INTEGRATION_SHARDS:-}"
weights_file="${INTEGRATION_PACKAGE_WEIGHTS:-scripts/integration-package-weights.tsv}"

if [ -z "$shard" ] && [ -z "$shards" ]; then
	cat
	exit 0
fi

if [ -z "$shard" ] || [ -z "$shards" ]; then
	echo "INTEGRATION_SHARD and INTEGRATION_SHARDS must be set together" >&2
	exit 2
fi

case "$shard" in
	*[!0-9]*)
		echo "invalid integration shard $shard/$shards" >&2
		exit 2
		;;
esac

case "$shards" in
	*[!0-9]*)
		echo "invalid integration shard $shard/$shards" >&2
		exit 2
		;;
esac

if [ "$shard" -lt 1 ] || [ "$shards" -lt 1 ] || [ "$shard" -gt "$shards" ]; then
	echo "invalid integration shard $shard/$shards" >&2
	exit 2
fi

if [ ! -f "$weights_file" ]; then
	echo "integration package weights not found: $weights_file" >&2
	exit 2
fi

awk -v selected_shard="$shard" -v shard_count="$shards" -v weights_file="$weights_file" '
	BEGIN {
		while ((getline line < weights_file) > 0) {
			sub(/\r$/, "", line)
			if (line ~ /^[[:space:]]*$/ || line ~ /^[[:space:]]*#/) {
				continue
			}
			fields = split(line, parts, /[[:space:]]+/)
			if (fields >= 2) {
				weights[parts[1]] = parts[2] + 0
			}
		}
		close(weights_file)
		for (bucket = 1; bucket <= shard_count; bucket++) {
			bucket_weight[bucket] = 0
		}
	}

	{
		packages[++package_count] = $0
		package_weight[package_count] = (($0 in weights) ? weights[$0] : 1)
	}

	END {
		for (assigned_count = 1; assigned_count <= package_count; assigned_count++) {
			best = 0
			for (idx = 1; idx <= package_count; idx++) {
				if (used[idx]) {
					continue
				}
				if (best == 0 ||
					package_weight[idx] > package_weight[best] ||
					(package_weight[idx] == package_weight[best] && packages[idx] < packages[best])) {
					best = idx
				}
			}

			target = 1
			for (bucket = 2; bucket <= shard_count; bucket++) {
				if (bucket_weight[bucket] < bucket_weight[target]) {
					target = bucket
				}
			}

			used[best] = 1
			assigned_shard[best] = target
			bucket_weight[target] += package_weight[best]
		}

		for (idx = 1; idx <= package_count; idx++) {
			if (assigned_shard[idx] == selected_shard) {
				print packages[idx]
			}
		}
	}
'
