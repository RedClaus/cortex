#!/usr/bin/env bash
set -euo pipefail

# new-app.sh - Scaffold a new plugin app

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APPS_DIR="${REPO_ROOT}/apps"
TEMPLATE_DIR="${APPS_DIR}/_template"

# Argument validation
if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <app-name>"
    echo "Example: $0 cortex-voice"
    exit 1
fi

APP_NAME="$1"

# Validate app name format (kebab-case)
if ! [[ $APP_NAME =~ ^[a-z][a-z0-9-]*$ ]]; then
    echo "Error: App name must be lowercase with hyphens (e.g., cortex-voice)"
    exit 1
fi

# Check if template exists
if [[ ! -d "$TEMPLATE_DIR" ]]; then
    echo "Error: Template directory not found at ${TEMPLATE_DIR}"
    exit 1
fi

# Check if app already exists
APP_DIR="${APPS_DIR}/${APP_NAME}"
if [[ -d "$APP_DIR" ]]; then
    echo "Error: App directory already exists at ${APP_DIR}"
    exit 1
fi

# Convert kebab-case to PascalCase for MODULE_NAME
# e.g., cortex-voice -> CortexVoice
MODULE_NAME=$(echo "$APP_NAME" | sed -r 's/(^|-)/\U/g; s/-//g')

# Convert to go module format (lowercase with hyphens)
GO_MODULE="github.com/cortexbrain/${APP_NAME}"

echo "Scaffolding new app: ${APP_NAME}"
echo "  Module name: ${MODULE_NAME}"
echo "  Go module: ${GO_MODULE}"
echo ""

# Copy template to new app directory
cp -r "$TEMPLATE_DIR" "$APP_DIR"
echo "✓ Copied template to ${APP_DIR}"

# Replace placeholders in all files
find "$APP_DIR" -type f -exec sed -i "s/MODULE_NAME/${MODULE_NAME}/g" {} +
find "$APP_DIR" -type f -exec sed -i "s|GO_MODULE|${GO_MODULE}|g" {} +
find "$APP_DIR" -type f -exec sed -i "s/APP_NAME/${APP_NAME}/g" {} +
echo "✓ Updated placeholders"

# Update go.mod in the new app
if [[ -f "${APP_DIR}/go.mod" ]]; then
    cat > "${APP_DIR}/go.mod" << EOF
module ${GO_MODULE}

go 1.23

require (
	github.com/cortexbrain/core v0.0.0
)

replace github.com/cortexbrain/core => ../../core
EOF
    echo "✓ Created go.mod"
fi

# Add to go.work
if [[ -f "${REPO_ROOT}/go.work" ]]; then
    # Check if already in go.work
    if ! grep -q "use ./apps/${APP_NAME}" "${REPO_ROOT}/go.work"; then
        # Add the new app to go.work
        sed -i "/^use /a use ./apps/${APP_NAME}" "${REPO_ROOT}/go.work"
        echo "✓ Added to go.work"
    fi
fi

# Run go work sync
echo "✓ Running go work sync..."
cd "$REPO_ROOT"
go work sync 2>/dev/null || echo "  (go work sync completed with status: $?)"

echo ""
echo "✓ Successfully scaffolded new app: ${APP_NAME}"
echo ""
echo "Next steps:"
echo "  1. cd ${APP_DIR}"
echo "  2. Update go.mod with actual dependencies"
echo "  3. Implement your app logic in cmd/ and internal/"
echo "  4. Add tests in internal/*/\*_test.go"
