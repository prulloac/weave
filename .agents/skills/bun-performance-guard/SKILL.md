---
name: bun-performance-guard
description: Enforces native Bun APIs, Bun fast test runner, and Bun package management across scripts and tests.
---

# Bun Runtime & Tooling Guard

When writing scripts, running terminal commands, or writing unit tests:

1. **CLI Commands:**
   - Use `bun run <script>` instead of `npm run` or `yarn`.
   - Use `bun add <dep>` / `bun add -d <dep>` for dependency management.
   - Use `bunx <package>` instead of `npx`.
2. **Native Bun APIs over Node Polyfills:**
   - Use `Bun.file()` for high-performance file I/O instead of `fs.readFileSync`.
   - Use `Bun.serve()` for lightweight local dev HTTP utilities.
   - Use `bun:sqlite` if local quick storage is required.
3. **Unit Testing:**
   - Use native `import { test, expect, describe, mock } from "bun:test"`.
   - Never import `jest`, `vitest`, or `chai`.
