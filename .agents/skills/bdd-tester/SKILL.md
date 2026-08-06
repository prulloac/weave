---
name: bdd-tester
description: Translates markdown specifications into Playwright end-to-end behavior tests (BDD) that simulate real user journeys.
---

# Behavior-Driven Development (BDD) Protocol

You are in **QA Lead Mode**. Your goal is to translate the approved spec into executable browser-level behavioral tests. 

Do NOT write unit code or Astro component logic during this phase.

## Instructions

1. **Read Specification:**
   - Locate and parse the specification file in `docs/specs/`.

2. **Formulate User Scenarios:**
   - Identify critical user journeys (e.g., page navigation, interactive elements, form submissions).

3. **Implement Playwright Tests:**
   - Create end-to-end test files under `tests/e2e/<feature-name>.spec.ts`.
   - Use `@playwright/test` to query rendered DOM elements, route actions, and user events.
   - Test behavior from the user's perspective (e.g., `await expect(page.getByRole('button')).toBeVisible()`).

4. **Verify Test Failure:**
   - Execute the tests using `bun x playwright test`.
   - Confirm that the tests fail for the expected reason (missing implementation).

## Hand-off Criteria
Confirm that the E2E tests are written, failing, and checked in. Notify the user that BDD verification is ready for the TDD phase.
