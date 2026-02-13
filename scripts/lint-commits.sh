#!/usr/bin/env bash
set -euo pipefail

# lint-commits.sh - Lint the last 10 commit messages against conventional commit format

# Valid types and scopes
VALID_TYPES=("feat" "fix" "docs" "refactor" "test" "ci" "chore" "perf")
VALID_SCOPES=("core" "pinky" "gateway" "coder" "avatar" "salamander" "lab" "dnet" "menu" "integrations" "adr" "research")

# Check if we're in a git repo
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "Error: Not a git repository"
    exit 1
fi

# Get the last 10 commit messages
mapfile -t commits < <(git log --format="%H %s" -10 | tac)

if [[ ${#commits[@]} -eq 0 ]]; then
    echo "No commits found"
    exit 0
fi

errors=0

for commit_line in "${commits[@]}"; do
    # Split hash and subject
    commit_hash=$(echo "$commit_line" | awk '{print $1}')
    commit_subject=$(echo "$commit_line" | cut -d' ' -f2-)

    # Validate format: type(scope): description or type: description
    if [[ ! $commit_subject =~ ^(feat|fix|docs|refactor|test|ci|chore|perf)(\([a-zA-Z0-9_-]+\))?: ]]; then
        echo "FAIL: ${commit_hash:0:7} - Invalid format: ${commit_subject}"
        ((errors++))
        continue
    fi

    # Extract type and scope
    type=$(echo "$commit_subject" | grep -oE '^[a-z]+' || true)
    scope=$(echo "$commit_subject" | grep -oE '^\w+\(([^)]+)\)' | grep -oE '\([^)]+\)' | tr -d '()' || true)

    # Validate type (should be guaranteed by regex but double check)
    type_found=0
    for valid_type in "${VALID_TYPES[@]}"; do
        if [[ "$type" == "$valid_type" ]]; then
            type_found=1
            break
        fi
    done

    if [[ $type_found -eq 0 ]]; then
        echo "FAIL: ${commit_hash:0:7} - Invalid type '${type}': ${commit_subject}"
        ((errors++))
        continue
    fi

    # Validate scope if present
    if [[ -n "$scope" ]]; then
        scope_found=0
        for valid_scope in "${VALID_SCOPES[@]}"; do
            if [[ "$scope" == "$valid_scope" ]]; then
                scope_found=1
                break
            fi
        done

        if [[ $scope_found -eq 0 ]]; then
            echo "FAIL: ${commit_hash:0:7} - Invalid scope '${scope}': ${commit_subject}"
            ((errors++))
            continue
        fi
    fi

    echo "PASS: ${commit_hash:0:7} - ${commit_subject}"
done

if [[ ${errors} -gt 0 ]]; then
    echo ""
    echo "Failed: ${errors} commit(s) do not follow conventional commit format"
    exit 1
else
    echo ""
    echo "Success: All commits follow conventional commit format"
    exit 0
fi
