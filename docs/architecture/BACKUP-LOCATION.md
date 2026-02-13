---
project: Cortex
component: Docs
phase: Experiment
date_created: 2026-01-15T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.576733
---

# Backup Archive Location

The pre-evolution backup archive has been moved to the Mac Mini for storage.

## Location

```
Host: 192.168.1.188 (Mac Mini)
Path: ~/cortex-temp-backup/cortex-old-environment.zip
Size: 7.7 GB
```

## Contents

The `cortex-old-environment.zip` contains the complete workspace state from **January 15, 2026** before the three-tier reorganization (Development/Test-UAT/Production structure).

## How to Retrieve

```bash
# Copy back to this machine
scp normanking@192.168.1.188:~/cortex-temp-backup/cortex-old-environment.zip .

# Or extract directly
ssh normanking@192.168.1.188 "cat ~/cortex-temp-backup/cortex-old-environment.zip" | unzip -
```

## Other Backups on Mac Mini

The following folders are also backed up at `192.168.1.188:~/cortex-temp-backup/`:

- `cortex-workshop/` (15 GB)
- `_archive/` (5.4 GB)
- `cortex-unified/` (5.0 GB)
- `gastown-workspace/` (3.0 GB)

---
*Created: 2026-01-15*
