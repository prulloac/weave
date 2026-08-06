---
name: tdd-coder
description: Executes Test-Driven Development (TDD) using Bun Test to write unit tests, followed by the minimal Astro/TypeScript code needed to pass all suites.
---

# Test-Driven Development (TDD) & Implementation Protocol

You are in **Implementation Engineer Mode**. Your goal is to write failing unit tests, craft the minimal Astro component/utility code to make them pass, and ensure both unit and BDD tests are green.

## 1. Input Handoff & Contract Retrieval
- **Read OKF Spec:** `docs/specs/<feature-id>.md`
  - Extract TypeScript interfaces from `## 3. Component Contracts (Props)`.
  - Extract component structure and hydration rules from `## 2. Component Structure & Hydration`.
- **Read BDD Suite:** `tests/e2e/<feature-id>.spec.ts`
  - Identify failing user assertion targets.

## 2. Red Phase (Unit Testing)
- **Target File Path:** `tests/unit/<feature-id>.test.ts`
- Write unit tests using native `bun:test` (`import { test, expect, describe } from "bun:test"`):
  - Test utility functions, data mapping, and isolated component helper logic.
- Run `bun test tests/unit/<feature-id>.test.ts` and confirm failure.

## 3. Green Phase (Implementation)
- Create or update `.astro` components, page routes, or TypeScript modules under `src/`.
- Adhere strictly to the `Props` interface exported in `docs/specs/<feature-id>.md`.
- Implement minimal code to satisfy tests—avoid unnecessary dependencies or over-engineering.

## 4. Verification & Green Checks
1. **Unit Verification:** Run `bun test tests/unit/<feature-id>.test.ts` (MUST PASS).
2. **BDD Verification:** Run `bun x playwright test tests/e2e/<feature-id>.spec.ts` (MUST PASS).

## 5. Refactor Phase
- Clean up formatting, imports, and types without altering external behavior or breaking green suites.

## 6. Handoff Output Protocol
Emit a completion summary once all suites pass:
- **Unit Suite:** `tests/unit/<feature-id>.test.ts` -> `GREEN`
- **BDD Suite:** `tests/e2e/<feature-id>.spec.ts` -> `GREEN`
- **Implementation Files:** List modified/created files in `src/`
- **Next Skill Signal:** Hand off to `git-stack-reviewer` to prepare stacked PR layer (`gh stack`).