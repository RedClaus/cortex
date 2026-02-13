.PHONY: test build lint sync check-work new-app new-adr changelog dep-check setup clean status

# ─── Build & Test ─────────────────────────────────────────────────────
# Run all tests across all Go modules in go.work
test:
	@echo "=== Testing all modules ==="
	@for mod in $$(grep -E '^\s+\./' go.work | sed 's|^\s*||' | grep -v '//'); do \
		echo "\n--- Testing $$mod ---"; \
		(cd "$$mod" && go test -race -count=1 ./...) || exit 1; \
	done
	@echo "\n=== All tests passed ==="

# Build all Go modules
build:
	@echo "=== Building all modules ==="
	@for mod in $$(grep -E '^\s+\./' go.work | sed 's|^\s*||' | grep -v '//'); do \
		echo "Building $$mod..."; \
		(cd "$$mod" && go build ./...) || exit 1; \
	done
	@echo "=== All builds succeeded ==="

# Build CortexBrain binary specifically
brain:
	cd core && go build -o /tmp/cortex ./cmd/cortex
	@echo "Binary: /tmp/cortex"

# Build CortexBrain server mode
brain-server:
	cd core && go build -o /tmp/cortex-server ./cmd/cortex-server
	@echo "Binary: /tmp/cortex-server"

# ─── Code Quality ────────────────────────────────────────────────────
# Lint commit messages (last 10 commits)
lint:
	@bash scripts/lint-commits.sh

# Check for illegal app-to-app dependencies
dep-check:
	@bash scripts/dep-check.sh

# ─── Workspace Management ────────────────────────────────────────────
# Sync go.work with all modules
sync:
	go work sync

# Verify go.work lists all modules
check-work:
	@bash scripts/check-go-work.sh

# ─── Scaffolding ─────────────────────────────────────────────────────
# Scaffold a new plugin: make new-app NAME=cortex-voice
new-app:
ifndef NAME
	$(error NAME is required. Usage: make new-app NAME=cortex-your-name)
endif
	@bash scripts/new-app.sh $(NAME)

# Create a new ADR: make new-adr TITLE="voicebox integration"
new-adr:
ifndef TITLE
	$(error TITLE is required. Usage: make new-adr TITLE="your decision title")
endif
	@bash scripts/new-adr.sh "$(TITLE)"

# Create a new research note: make new-research TOPIC="vector-db-benchmarks"
new-research:
ifndef TOPIC
	$(error TOPIC is required. Usage: make new-research TOPIC="your-topic-slug")
endif
	@cp research/_template.md "research/$$(date +%Y-%m-%d)-$(TOPIC).md"
	@echo "Created: research/$$(date +%Y-%m-%d)-$(TOPIC).md"

# ─── Changelog ────────────────────────────────────────────────────────
# Generate changelog from conventional commits (requires git-chglog)
changelog:
	git-chglog -o CHANGELOG.md

# ─── Info ─────────────────────────────────────────────────────────────
# List all modules in the workspace
list:
	@echo "=== Go modules in workspace ==="
	@grep -E '^\s+\./' go.work | sed 's|^\s*||' | grep -v '//'
	@echo ""
	@echo "=== Non-Go projects ==="
	@ls -d apps/*/pyproject.toml apps/*/package.json 2>/dev/null | sed 's|/[^/]*$$||' || echo "(none yet)"

# ─── Setup & Maintenance ────────────────────────────────────────────
# Install git hooks
setup:
	@bash scripts/setup-hooks.sh

# Remove original directories that were migrated (interactive)
clean:
	@bash scripts/cleanup-originals.sh

# Show workspace status
status:
	@echo "=== Cortex Monorepo Status ==="
	@echo ""
	@echo "Modules in workspace:"
	@grep -E '^\s+\./' go.work | sed 's|^\s*||' | grep -v '//' | while read mod; do \
		if [ -f "$$mod/go.mod" ]; then \
			echo "  ✅ $$mod"; \
		else \
			echo "  ❌ $$mod (go.mod missing!)"; \
		fi; \
	done
	@echo ""
	@echo "Archive entries:"
	@ls -d archive/*/ 2>/dev/null | wc -l | xargs -I{} echo "  {} archived directories"
	@echo ""
	@echo "ADRs:"
	@ls docs/adr/*.md 2>/dev/null | grep -v _template | wc -l | xargs -I{} echo "  {} decisions recorded"

# Show help
help:
	@echo "Cortex Monorepo Commands:"
	@echo ""
	@echo "  make build         Build all Go modules"
	@echo "  make test          Test all Go modules"
	@echo "  make brain         Build CortexBrain binary → /tmp/cortex"
	@echo "  make brain-server  Build CortexBrain server → /tmp/cortex-server"
	@echo "  make lint          Lint recent commit messages"
	@echo "  make dep-check     Check for illegal dependencies"
	@echo "  make sync          Sync go.work"
	@echo "  make check-work    Verify go.work is complete"
	@echo "  make new-app       Scaffold new plugin (NAME=...)"
	@echo "  make new-adr       Create new ADR (TITLE=...)"
	@echo "  make new-research  Create research note (TOPIC=...)"
	@echo "  make changelog     Generate CHANGELOG.md"
	@echo "  make list          List all modules"
	@echo "  make setup         Install git hooks"
	@echo "  make status        Show workspace health"
	@echo "  make clean         Remove migrated originals (interactive)"
