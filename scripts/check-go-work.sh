#!/usr/bin/env bash
set -euo pipefail

# check-go-work.sh - Verify that every Go module directory has a corresponding entry in go.work

# Find the repository root (where go.work is located)
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_WORK_FILE="${REPO_ROOT}/go.work"

if [[ ! -f "${GO_WORK_FILE}" ]]; then
    echo "Error: go.work file not found at ${GO_WORK_FILE}"
    exit 1
fi

# Find all go.mod files, excluding vendor/
mapfile -t GO_MODULES < <(find "${REPO_ROOT}" -name "go.mod" -not -path "*/vendor/*" | sort)

if [[ ${#GO_MODULES[@]} -eq 0 ]]; then
    echo "No go.mod files found"
    exit 0
fi

errors=0

for go_mod in "${GO_MODULES[@]}"; do
    # Get the directory containing go.mod
    module_dir=$(dirname "$go_mod")
    # Get relative path from repo root
    rel_path=$(realpath --relative-to="${REPO_ROOT}" "$module_dir")

    # Check if this path appears in go.work (either as "use" or commented)
    if ! grep -E "^[[:space:]]*(//[[:space:]]*)?use[[:space:]]+\./?(${rel_path}|${rel_path}/)" "${GO_WORK_FILE}" > /dev/null; then
        echo "Error: Module directory '${rel_path}' not found in go.work"
        ((errors++))
    fi
done

if [[ ${errors} -gt 0 ]]; then
    echo "Failed: ${errors} module(s) not listed in go.work"
    exit 1
else
    echo "Success: All $(echo ${#GO_MODULES[@]}) modules are listed in go.work"
    exit 0
fi
