---
name: systematic-debugging
description: Enforces a structured, diagnostic debugging workflow before making code changes when Go tests or builds fail.
---

# Systematic Debugging Protocol

When a Go test, build, contract export, or CLI process test fails:

1. **Root Cause Analysis (Do NOT edit code yet):**
   - Read the exact failure output, stack trace, golden diff or exit code.
   - Formulate a single hypothesis for why it failed.
2. **Isolate:**
   - Run only the failing package or test:
     ```sh
     go test ./engine/search -run TestCalibration -v
     go test ./cmd/weave -run TestSearch -v
     ```
   - Inspect the smallest input/output boundary.
3. **Minimal Fix:**
   - Apply the smallest change that addresses the root cause.
   - Do not weaken tests or calibration baselines.
4. **Regression Verification:**
   ```sh
   go test ./...
   go test -race ./...
   ./scripts/verify.sh
   ```
