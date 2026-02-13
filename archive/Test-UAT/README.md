---
project: Cortex
component: Docs
phase: Build
date_created: 2026-01-15T19:26:21
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:30.088223
---

# Test-UAT Environment

User Acceptance Testing before production deployment.

## Purpose

This environment is for:
- Integration testing
- User acceptance testing
- Performance validation
- Bug verification before production release

## Workflow

### 1. Copy from Development
```bash
cp -r /Users/normanking/ServerProjectsMac/Development/<app-name> \
      /Users/normanking/ServerProjectsMac/Test-UAT/<app-name>
```

### 2. Run Tests
Execute the application and verify:
- Core functionality works as expected
- No regressions from previous version
- Performance is acceptable
- Edge cases are handled

### 3. Testing Checklist

Before promoting to Production, verify:

- [ ] Application builds successfully
- [ ] All unit tests pass
- [ ] Manual smoke test passes
- [ ] No critical bugs identified
- [ ] Performance is acceptable
- [ ] Security considerations reviewed
- [ ] Documentation updated if needed

### 4. Promote to Production
If all tests pass:
```bash
cp -r /Users/normanking/ServerProjectsMac/Test-UAT/<app-name> \
      /Users/normanking/ServerProjectsMac/Production/<app-name>
```

### 5. Cleanup
After successful production deployment, remove from Test-UAT:
```bash
rm -rf /Users/normanking/ServerProjectsMac/Test-UAT/<app-name>
```

## Current Tests

| App | Test Status | Notes |
|-----|-------------|-------|
| (empty) | - | No apps currently in testing |

## Testing Best Practices

1. **Isolate Tests** - Test one change at a time
2. **Document Issues** - Record any bugs found
3. **Verify Fixes** - Re-test after bug fixes
4. **Clean State** - Start with fresh data when possible
5. **Multiple Scenarios** - Test happy path and edge cases
