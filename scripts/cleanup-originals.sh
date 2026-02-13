#!/bin/bash

################################################################################
# cleanup-originals.sh
#
# Safe removal script for archived directories.
# Use this script AFTER confirming the cortex monorepo is working correctly.
#
# This script:
# - Lists each directory to be removed with its size
# - Asks for confirmation before each deletion
# - Uses 'trash' command (macOS) if available, otherwise 'rm -rf'
# - Provides rollback suggestions
#
# USAGE:
#   chmod +x cortex/scripts/cleanup-originals.sh
#   ./cortex/scripts/cleanup-originals.sh
#
################################################################################

set -e

BASE_PATH="${1:-.}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCHIVE_DIR="$(dirname "$SCRIPT_DIR")/archive"

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if trash command exists (macOS)
USE_TRASH=false
if command -v trash &> /dev/null; then
  USE_TRASH=true
fi

# Banner
echo -e "${BLUE}┌─────────────────────────────────────────────────────────────┐${NC}"
echo -e "${BLUE}│  Cortex Monorepo Cleanup Script                             │${NC}"
echo -e "${BLUE}│  Safe Removal of Test Variants & Archived Directories       │${NC}"
echo -e "${BLUE}└─────────────────────────────────────────────────────────────┘${NC}"
echo ""

# Directories to remove (with descriptions)
declare -A DIRS_TO_REMOVE=(
  # Test Variants
  ["cortex-brain-test"]="Test copy of CortexBrain (397M) → original: cortex/core/cortex-brain/"
  ["cortex-brain-git"]="Git experiment copy (143M) → original: cortex/core/cortex-brain/"
  ["Pinky-Test"]="Test copy of Pinky (30M) → original: cortex/core/pinky/"
  ["cortex-gateway-test"]="Test copy (232M) → original: cortex/apps/cortex-gateway/"
  ["cortex-coder-agent-test"]="Test copy (19M) → original: cortex/apps/cortex-coder-agent/"
  ["Test-UAT"]="UAT test environment (12K)"
  ["cortex-teacher"]="Experimental, no go.mod (52K)"
  ["claude_code_agent"]="Legacy code agent (342M)"
)

# Development folder early versions
declare -A DEV_DIRS_TO_REMOVE=(
  ["Development/cortex-02"]="Earlier CortexBrain v2 iteration (1.5GB+)"
  ["Development/cortex-03"]="Earlier CortexBrain v3 iteration (1.8GB+)"
  ["Development/cortex-workshop"]="Experimental workshop (100MB+)"
  ["Development/cortex-unified"]="Merge experiment (100MB+)"
  ["Development/cortex-assistant"]="Earlier assistant concept (200MB+)"
  ["Development/TEST-Cortex-Assistant"]="Test variant (100MB+)"
  ["Development/gotui-sandbox"]="TUI experiments (100MB+)"
)

# Production folder
declare -A PROD_DIRS_TO_REMOVE=(
  ["Production/cortex-key-vault"]="Key vault service → original: cortex/apps/cortex-key-vault/"
)

# Root level old projects
declare -A ROOT_DIRS_TO_REMOVE=(
  ["CortexBrain"]="Old CortexBrain folder → original: cortex/core/cortex-brain/"
  ["Pinky"]="Old Pinky folder → original: cortex/core/pinky/"
)

################################################################################
# Functions
################################################################################

verify_original_exists() {
  local original="$1"
  if [ ! -d "$BASE_PATH/$original" ]; then
    echo -e "${RED}✗ Original not found: $original${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Original verified: $original${NC}"
  return 0
}

get_size() {
  local path="$1"
  if [ -d "$path" ]; then
    du -sh "$path" 2>/dev/null | cut -f1
  else
    echo "N/A"
  fi
}

confirm_deletion() {
  local dir_name="$1"
  local description="$2"
  local size=$(get_size "$dir_name")

  echo ""
  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "Directory: ${BLUE}$dir_name${NC}"
  echo -e "Description: $description"
  echo -e "Size: ${YELLOW}$size${NC}"
  echo ""

  if [ ! -d "$dir_name" ]; then
    echo -e "${YELLOW}⚠ Directory not found (may already be deleted)${NC}"
    return 2
  fi

  while true; do
    read -p "Delete this directory? (yes/no/skip): " choice
    case "$choice" in
      yes|y)
        return 0
        ;;
      no|n)
        echo -e "${YELLOW}Skipping deletion.${NC}"
        return 1
        ;;
      skip|s)
        echo -e "${YELLOW}Skipping this directory.${NC}"
        return 1
        ;;
      *)
        echo "Please answer 'yes', 'no', or 'skip'."
        ;;
    esac
  done
}

delete_directory() {
  local dir_name="$1"

  if [ ! -d "$dir_name" ]; then
    echo -e "${YELLOW}Directory not found: $dir_name${NC}"
    return 1
  fi

  echo -e "${BLUE}Deleting: $dir_name${NC}"

  if [ "$USE_TRASH" = true ]; then
    trash "$dir_name"
    echo -e "${GREEN}✓ Moved to trash: $dir_name${NC}"
  else
    rm -rf "$dir_name"
    echo -e "${GREEN}✓ Permanently deleted: $dir_name${NC}"
  fi

  return 0
}

################################################################################
# Main Cleanup Process
################################################################################

TOTAL_DELETED=0
TOTAL_SKIPPED=0
TOTAL_NOT_FOUND=0

# Phase 1: Small Test Variants (Safe to delete)
echo -e "${BLUE}PHASE 1: Small Test Variants (Safe to Delete)${NC}"
echo -e "These are clearly test copies with originals in cortex/."
echo ""

cd "$BASE_PATH"

for dir in "${!DIRS_TO_REMOVE[@]}"; do
  confirm_deletion "$dir" "${DIRS_TO_REMOVE[$dir]}"
  result=$?

  if [ $result -eq 0 ]; then
    if delete_directory "$dir"; then
      TOTAL_DELETED=$((TOTAL_DELETED + 1))
    fi
  elif [ $result -eq 1 ]; then
    TOTAL_SKIPPED=$((TOTAL_SKIPPED + 1))
  else
    TOTAL_NOT_FOUND=$((TOTAL_NOT_FOUND + 1))
  fi
done

# Phase 2: Development Folder Early Versions (Optional)
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}PHASE 2: Development Folder Early Versions (Optional)${NC}"
echo -e "These are larger (1-2GB each) and superseded by CortexBrain v2+."
read -p "Proceed with Development folder cleanup? (yes/no): " proceed_dev
if [ "$proceed_dev" = "yes" ] || [ "$proceed_dev" = "y" ]; then
  for dir in "${!DEV_DIRS_TO_REMOVE[@]}"; do
    confirm_deletion "$dir" "${DEV_DIRS_TO_REMOVE[$dir]}"
    result=$?

    if [ $result -eq 0 ]; then
      if delete_directory "$dir"; then
        TOTAL_DELETED=$((TOTAL_DELETED + 1))
      fi
    elif [ $result -eq 1 ]; then
      TOTAL_SKIPPED=$((TOTAL_SKIPPED + 1))
    else
      TOTAL_NOT_FOUND=$((TOTAL_NOT_FOUND + 1))
    fi
  done
else
  echo -e "${YELLOW}Skipping Development folder cleanup.${NC}"
fi

# Phase 3: Production Folder (Optional)
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}PHASE 3: Production Folder (Optional)${NC}"
echo -e "Legacy services that have been migrated."
read -p "Proceed with Production folder cleanup? (yes/no): " proceed_prod
if [ "$proceed_prod" = "yes" ] || [ "$proceed_prod" = "y" ]; then
  for dir in "${!PROD_DIRS_TO_REMOVE[@]}"; do
    confirm_deletion "$dir" "${PROD_DIRS_TO_REMOVE[$dir]}"
    result=$?

    if [ $result -eq 0 ]; then
      if delete_directory "$dir"; then
        TOTAL_DELETED=$((TOTAL_DELETED + 1))
      fi
    elif [ $result -eq 1 ]; then
      TOTAL_SKIPPED=$((TOTAL_SKIPPED + 1))
    else
      TOTAL_NOT_FOUND=$((TOTAL_NOT_FOUND + 1))
    fi
  done
else
  echo -e "${YELLOW}Skipping Production folder cleanup.${NC}"
fi

# Phase 4: Root Level Old Projects (Optional)
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}PHASE 4: Root Level Old Projects (Optional)${NC}"
echo -e "Large consolidated projects now in cortex/core/"
read -p "Proceed with root level cleanup? (yes/no): " proceed_root
if [ "$proceed_root" = "yes" ] || [ "$proceed_root" = "y" ]; then
  for dir in "${!ROOT_DIRS_TO_REMOVE[@]}"; do
    confirm_deletion "$dir" "${ROOT_DIRS_TO_REMOVE[$dir]}"
    result=$?

    if [ $result -eq 0 ]; then
      if delete_directory "$dir"; then
        TOTAL_DELETED=$((TOTAL_DELETED + 1))
      fi
    elif [ $result -eq 1 ]; then
      TOTAL_SKIPPED=$((TOTAL_SKIPPED + 1))
    else
      TOTAL_NOT_FOUND=$((TOTAL_NOT_FOUND + 1))
    fi
  done
else
  echo -e "${YELLOW}Skipping root level cleanup.${NC}"
fi

################################################################################
# Summary
################################################################################

echo ""
echo -e "${BLUE}┌─────────────────────────────────────────────────────────────┐${NC}"
echo -e "${BLUE}│  Cleanup Summary                                            │${NC}"
echo -e "${BLUE}└─────────────────────────────────────────────────────────────┘${NC}"
echo -e "Deleted:     ${GREEN}$TOTAL_DELETED directories${NC}"
echo -e "Skipped:     ${YELLOW}$TOTAL_SKIPPED directories${NC}"
echo -e "Not found:   ${YELLOW}$TOTAL_NOT_FOUND directories${NC}"
echo ""

if [ $TOTAL_DELETED -gt 0 ]; then
  echo -e "${GREEN}✓ Successfully cleaned up $TOTAL_DELETED directories.${NC}"
  echo ""
  if [ "$USE_TRASH" = true ]; then
    echo -e "${BLUE}Note:${NC} Directories were moved to Trash. Empty Trash to reclaim space."
  else
    echo -e "${YELLOW}Warning:${NC} Directories were permanently deleted. Consider backing up if needed."
  fi
else
  echo -e "${YELLOW}No directories were deleted.${NC}"
fi

echo ""
echo -e "${BLUE}For more information, see:${NC} cortex/archive/ARCHIVE-MANIFEST.md"
echo ""
