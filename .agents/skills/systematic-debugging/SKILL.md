---
name: systematic-debugging
description: Enforces a structured, diagnostic debugging workflow before making code changes when tests fail.
---

# Systematic Debugging Protocol

When a test (`bun test` or Playwright) fails or an exception occurs:

1. **Root Cause Analysis (Do NOT edit code yet):**
   - Read the exact stack trace and failure output.
   - Formulate a single hypothesis on *why* it failed.
2. **Isolate:**
   - Run only the specific failing test case (`bun test -t "test name"`).
   - Log or inspect inputs/outputs at the exact failure boundary.
3. **Minimal Fix:**
   - Apply the smallest fix directly addressing the root cause.
   - Re-run the isolated test.
4. **Regression Verification:**
   - Run the entire test suite to ensure no collateral damage was introduced.
