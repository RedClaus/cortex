---
project: Cortex
component: Docs
phase: Archive
date_created: 2026-01-16T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:20:27.539235
---

# Gas Town Plugins

This directory contains town-level plugins that run during Deacon patrol cycles.

## Plugin Structure

Each plugin is a directory containing:
- plugin.md - Plugin definition with TOML frontmatter

## Gate Types

- cooldown: Time since last run (e.g., 24h)
- cron: Schedule-based (e.g., "0 9 * * *")
- condition: Metric threshold
- event: Trigger-based (startup, heartbeat)

See docs/deacon-plugins.md for full documentation.
