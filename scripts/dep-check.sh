#!/usr/bin/env bash
set -euo pipefail

# dep-check.sh - Check for illegal app-to-app dependencies
# Rule: apps/* can import core/ and apps/cortex-lab/pkg/ but NOT any other app

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APPS_DIR="${REPO_ROOT}/apps"

# Check if apps directory exists
if [[ ! -d "${APPS_DIR}" ]]; then
    echo "No apps to check"
    exit 0
fi

errors=0

# Find all directories under apps/
for app_dir in "${APPS_DIR}"/*/; do
    app_name=$(basename "$app_dir")

    # Skip template directory
    if [[ "$app_name" == "_template" ]]; then
        continue
    fi

    # Skip if no go.mod (not a Go app)
    if [[ ! -f "${app_dir}/go.mod" ]]; then
        continue
    fi

    # Find all Go files in this app
    mapfile -t go_files < <(find "$app_dir" -name "*.go" -type f)

    for go_file in "${go_files[@]}"; do
        # Extract import statements (simplified - looks for common patterns)
        # We're looking for imports that reference other apps
        mapfile -t imports < <(grep -h '^\s*".*"' "$go_file" | grep -E '^\s*"' | sed 's/.*"\(.*\)".*/\1/' || true)

        for import in "${imports[@]}"; do
            # Check if this import is from another app (but not core or cortex-lab/pkg)
            if [[ $import =~ ^.*/(apps/[^/]+) ]]; then
                referenced_app="${BASH_REMATCH[1]}"
                referenced_app_name=$(echo "$referenced_app" | sed 's|apps/||')

                # Allow imports from core and cortex-lab/pkg
                if [[ "$referenced_app_name" == "core" ]] || [[ "$referenced_app_name" == "cortex-lab" && $import =~ /pkg/ ]]; then
                    continue
                fi

                # Check if it's a different app (illegal dependency)
                if [[ "$referenced_app_name" != "$app_name" ]]; then
                    echo "Error: ${app_name} imports from ${referenced_app_name} (illegal app-to-app dependency)"
                    echo "  File: ${go_file#${REPO_ROOT}/}"
                    echo "  Import: ${import}"
                    ((errors++))
                fi
            fi
        done
    done
done

if [[ ${errors} -gt 0 ]]; then
    echo ""
    echo "Failed: ${errors} illegal app-to-app dependency(ies) found"
    exit 1
else
    echo "Success: No illegal app-to-app dependencies found"
    exit 0
fi
