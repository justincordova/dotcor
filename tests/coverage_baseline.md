# Coverage Baseline

**Date:** 2026-02-10
**Status:** Pre-comprehensive-testing implementation
**Target Release:** v0.6.0

## Baseline Coverage

**Total Coverage:** 27.4%

### Package-Specific Coverage

| Package | Coverage | Statements |
|---------|----------|------------|
| cmd/dotcor | 8.2% | - |
| internal/config | 64.4% | - |
| internal/core | 50.1% | - |
| internal/fs | 69.7% | - |
| internal/git | 56.1% | - |
| internal/logger | 50.0% | - |
| tests | 0.0% | - |

## Goals

**Target Coverage:** 75%+ for all commands
**Target Lines:** 7,400+ total test lines (from ~4,017 baseline)

## Next Steps

1. Expand command test files to target line counts
2. Add integration tests for core workflows
3. Achieve 75%+ coverage per command
4. Verify with golangci-lint and race detector

## Commands to Check Coverage

```bash
# Generate coverage
go test ./... -coverprofile=coverage.out -covermode=atomic

# Get total percentage
go tool cover -func=coverage.out | tail -1

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Compare with baseline
go tool cover -func=coverage.out > coverage.txt
go tool cover -func=coverage_baseline.out > baseline.txt
diff coverage.txt baseline.txt
```
