#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'USAGE'
Usage: scripts/run-affected-tests.sh [options]

Runs the test commands most directly affected by the current code change.

Options:
  --base REF           Compare changes against REF. Defaults to the merge-base
                       with origin/main, main, or master.
  --head REF           Compare changes up to REF. Defaults to HEAD.
  --changed-file PATH  Use an explicit changed file. Can be repeated. When set,
                       git diff discovery is skipped.
  --list               Print selected commands without running them.
  -h, --help           Show this help.

Environment:
  AFFECTED_BASE        Same as --base.
  AFFECTED_HEAD        Same as --head.
USAGE
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

base_ref="${AFFECTED_BASE:-}"
head_ref="${AFFECTED_HEAD:-HEAD}"
list_only=false
explicit_changed=false

changed_files_tmp="$(mktemp)"
commands_tmp="$(mktemp)"
changed_imports_tmp="$(mktemp)"
affected_go_tmp="$(mktemp)"
frontend_e2e_tmp="$(mktemp)"
trap 'rm -f "$changed_files_tmp" "$commands_tmp" "$changed_imports_tmp" "$affected_go_tmp" "$frontend_e2e_tmp"' EXIT

while [ "$#" -gt 0 ]; do
	case "$1" in
		--base)
			if [ "$#" -lt 2 ]; then
				echo "--base requires a value" >&2
				exit 2
			fi
			base_ref="$2"
			shift 2
			;;
		--head)
			if [ "$#" -lt 2 ]; then
				echo "--head requires a value" >&2
				exit 2
			fi
			head_ref="$2"
			shift 2
			;;
		--changed-file)
			if [ "$#" -lt 2 ]; then
				echo "--changed-file requires a value" >&2
				exit 2
			fi
			explicit_changed=true
			printf '%s\n' "$2" >> "$changed_files_tmp"
			shift 2
			;;
		--list)
			list_only=true
			shift
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "unknown option: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

resolve_default_base() {
	if [ -n "$base_ref" ]; then
		printf '%s\n' "$base_ref"
		return
	fi

	for candidate in origin/main main master; do
		if git rev-parse --verify --quiet "$candidate" >/dev/null; then
			git merge-base "$head_ref" "$candidate"
			return
		fi
	done

	printf '\n'
}

default_base="$(resolve_default_base)"
if [ -z "$base_ref" ]; then
	base_ref="$default_base"
fi

if [ "$explicit_changed" = false ]; then
	if [ -n "$base_ref" ]; then
		git diff --name-only "$base_ref...$head_ref" >> "$changed_files_tmp"
	else
		git diff --name-only "$head_ref" >> "$changed_files_tmp"
	fi
	git diff --name-only >> "$changed_files_tmp"
	git diff --cached --name-only >> "$changed_files_tmp"
	git ls-files --others --exclude-standard -- .github cmd docs frontend internal migrations scripts Makefile README.md go.mod go.sum 2>/dev/null >> "$changed_files_tmp"
fi

sort -u "$changed_files_tmp" | sed '/^$/d' > "$changed_files_tmp.sorted"
mv "$changed_files_tmp.sorted" "$changed_files_tmp"

if [ ! -s "$changed_files_tmp" ]; then
	echo "No changed files; no affected tests selected."
	exit 0
fi

add_command() {
	printf '%s\n' "$*" >> "$commands_tmp"
}

shell_quote() {
	printf '%q' "$1"
}

nearest_go_package_import() {
	path="$1"
	dir="$(dirname "$path")"

	while [ "$dir" != "." ] && [ "$dir" != "/" ] && [ -n "$dir" ]; do
		if find "$dir" -maxdepth 1 -name '*.go' -type f 2>/dev/null | grep -q .; then
			go list "./$dir" 2>/dev/null || true
			return
		fi
		dir="$(dirname "$dir")"
	done
}

module_path="$(go list -m 2>/dev/null || true)"

import_to_package_arg() {
	import_path="$1"
	case "$import_path" in
		"$module_path")
			printf '.\n'
			;;
		"$module_path"/*)
			printf './%s\n' "${import_path#"$module_path"/}"
			;;
		*)
			printf '%s\n' "$import_path"
			;;
	esac
}

full_backend=false
docs_required=false
frontend_changed=false
frontend_full=false
cli_coverage_required=false
migration_required=false

while IFS= read -r file; do
	case "$file" in
		go.mod|go.sum)
			full_backend=true
			;;
		migrations/*.sql)
			migration_required=true
			;;
		README.md|docs/*|scripts/README.md|scripts/integration-package-weights.tsv|Makefile)
			docs_required=true
			;;
		.github/workflows/*|scripts/*.sh|scripts/*.mjs)
			docs_required=true
			;;
	esac

	case "$file" in
		*.go|cmd/*|internal/*)
			pkg_import="$(nearest_go_package_import "$file")"
			if [ -n "$pkg_import" ]; then
				printf '%s\n' "$pkg_import" >> "$changed_imports_tmp"
			fi
			;;
	esac

	case "$file" in
		frontend/package.json|frontend/bun.lock|frontend/vitest.config.*|frontend/vite.config.*|frontend/svelte.config.*|frontend/tsconfig*.json)
			frontend_full=true
			;;
		frontend/e2e/*.spec.ts|frontend/e2e/**/*.spec.ts)
			frontend_changed=true
			printf '%s\n' "${file#frontend/}" >> "$frontend_e2e_tmp"
			;;
		frontend/src/*|frontend/src/**/*|frontend/messages/*|frontend/project.inlang/*|frontend/*.ts|frontend/*.js|frontend/*.json)
			frontend_changed=true
			;;
	esac
done < "$changed_files_tmp"

if [ "$full_backend" = true ]; then
	add_command "make test-backend-coverage"
else
	sort -u "$changed_imports_tmp" | sed '/^$/d' > "$changed_imports_tmp.sorted"
	mv "$changed_imports_tmp.sorted" "$changed_imports_tmp"

	if [ -s "$changed_imports_tmp" ]; then
		all_packages_tmp="$(mktemp)"
		trap 'rm -f "$changed_files_tmp" "$commands_tmp" "$changed_imports_tmp" "$affected_go_tmp" "$frontend_e2e_tmp" "$all_packages_tmp"' EXIT
		go list ./... > "$all_packages_tmp"

		while IFS= read -r pkg; do
			deps="$(go list -deps -test "$pkg" 2>/dev/null || true)"
			while IFS= read -r changed_import; do
				if [ "$pkg" = "$changed_import" ] || printf '%s\n' "$deps" | grep -Fxq "$changed_import"; then
					import_to_package_arg "$pkg" >> "$affected_go_tmp"
					break
				fi
			done < "$changed_imports_tmp"
		done < "$all_packages_tmp"

		sort -u "$affected_go_tmp" | sed '/^$/d' > "$affected_go_tmp.sorted"
		mv "$affected_go_tmp.sorted" "$affected_go_tmp"

		if [ -s "$affected_go_tmp" ]; then
			packages="$(tr '\n' ' ' < "$affected_go_tmp" | sed 's/[[:space:]]*$//')"
			add_command "go test -count=1 -race $packages"
			if grep -Fxq './cmd/oa' "$affected_go_tmp"; then
				cli_coverage_required=true
			fi
		fi
	fi
fi

if [ "$docs_required" = true ] && ! grep -Fxq './docs' "$affected_go_tmp" 2>/dev/null; then
	add_command "go test -timeout=3m ./docs -count=1"
fi

if [ "$cli_coverage_required" = true ]; then
	add_command "make test-cli-coverage"
fi

if [ "$migration_required" = true ]; then
	add_command "go test -timeout=5m -tags=integration ./cmd/migrate -count=1"
fi

if [ "$frontend_full" = true ]; then
	add_command "cd frontend && bun run paraglide && bun run test:prepared"
elif [ "$frontend_changed" = true ]; then
	if [ -n "$base_ref" ]; then
		add_command "cd frontend && bun run paraglide && bun run test:prepared -- --changed $(shell_quote "$base_ref")"
	else
		add_command "cd frontend && bun run paraglide && bun run test:prepared -- --changed"
	fi
fi

if [ -s "$frontend_e2e_tmp" ]; then
	sort -u "$frontend_e2e_tmp" > "$frontend_e2e_tmp.sorted"
	mv "$frontend_e2e_tmp.sorted" "$frontend_e2e_tmp"
	specs=""
	while IFS= read -r spec; do
		specs="$specs $(shell_quote "$spec")"
	done < "$frontend_e2e_tmp"
	add_command "cd frontend && bunx playwright test --config=playwright.demo.config.ts --project=demo-chromium$specs"
fi

sort -u "$commands_tmp" > "$commands_tmp.sorted"
mv "$commands_tmp.sorted" "$commands_tmp"

if [ ! -s "$commands_tmp" ]; then
	echo "No affected test commands selected for changed files:"
	sed 's/^/  /' "$changed_files_tmp"
	exit 0
fi

if [ "$list_only" = true ]; then
	cat "$commands_tmp"
	exit 0
fi

while IFS= read -r command; do
	echo "+ $command"
	bash -lc "$command"
done < "$commands_tmp"
