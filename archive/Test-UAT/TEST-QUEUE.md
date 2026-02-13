---
project: Cortex
component: Unknown
phase: Build
date_created: 2026-01-15T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:30.098156
---

# Test-UAT Queue

> Last updated: 2026-01-15 21:10:54

## Purpose

This environment is for **User Acceptance Testing** before production promotion.
Apps here have passed development but need validation before going live.

## Current Queue

| App | Source Version | Test Started | Tester | Status |
|-----|----------------|--------------|--------|--------|
| (empty) | - | - | - | - |

## Testing Checklist Template

Before promoting to Production, verify:

### Functionality
- [ ] Core features work as expected
- [ ] Edge cases handled properly
- [ ] Error messages are clear

### Performance
- [ ] Acceptable startup time
- [ ] Memory usage reasonable
- [ ] No performance regressions

### Documentation
- [ ] PROJECT.md is accurate
- [ ] CHANGELOG.md is updated
- [ ] Help/usage text is correct

### Security
- [ ] No hardcoded secrets
- [ ] Input validation in place
- [ ] Proper error handling

## Promotion Command

When testing is complete:

```bash
./Evolve-Now.sh promote <app> --from test-uat --to production --version X.Y.Z
```

---
*Maintained by Evolve-Now v3.0*
