#!/usr/bin/env bash
set -euo pipefail

# new-adr.sh - Create a new ADR file

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADR_DIR="${REPO_ROOT}/docs/adr"
TEMPLATE_FILE="${ADR_DIR}/_template.md"

# Argument validation
if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <title>"
    echo "Example: $0 'voicebox integration'"
    exit 1
fi

TITLE="$1"

# Check if ADR directory exists
if [[ ! -d "$ADR_DIR" ]]; then
    echo "Error: ADR directory not found at ${ADR_DIR}"
    exit 1
fi

# Check if template exists
if [[ ! -f "$TEMPLATE_FILE" ]]; then
    echo "Error: Template file not found at ${TEMPLATE_FILE}"
    exit 1
fi

# Find the next ADR number
# Look for existing ADR files in format NNNN-*.md
latest_num=0
for adr_file in "$ADR_DIR"/[0-9][0-9][0-9][0-9]-*.md; do
    if [[ -f "$adr_file" ]]; then
        filename=$(basename "$adr_file")
        num=$(echo "$filename" | grep -oE '^[0-9]+' || true)
        if [[ -n "$num" ]]; then
            # Remove leading zeros and convert to number
            num=$((10#$num))
            if [[ $num -gt $latest_num ]]; then
                latest_num=$num
            fi
        fi
    fi
done

next_num=$((latest_num + 1))
# Pad with leading zeros to 4 digits
next_num_padded=$(printf "%04d" $next_num)

# Convert title to slug (lowercase, spaces to hyphens, remove special chars)
slug=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | sed 's/[^a-z0-9-]//g' | sed 's/-+/-/g')

# Create filename
adr_filename="${next_num_padded}-${slug}.md"
adr_filepath="${ADR_DIR}/${adr_filename}"

# Check if file already exists
if [[ -f "$adr_filepath" ]]; then
    echo "Error: ADR file already exists at ${adr_filepath}"
    exit 1
fi

# Get current date in format YYYY-MM-DD
current_date=$(date +%Y-%m-%d)

# Copy template and replace placeholders
cp "$TEMPLATE_FILE" "$adr_filepath"

# Replace placeholders
sed -i "s/NNNN/${next_num_padded}/g" "$adr_filepath"
sed -i "s/YYYY-MM-DD/${current_date}/g" "$adr_filepath"
sed -i "s/Title/${TITLE}/g" "$adr_filepath"

echo "✓ Created ADR: ${adr_filename}"
echo ""
echo "Next steps:"
echo "  1. Edit: ${adr_filepath}"
echo "  2. Fill in Status, Context, Decision, and Consequences sections"
echo "  3. Commit with: git add docs/adr/ && git commit -m 'docs(adr): ADR ${next_num_padded} - ${slug}'"
