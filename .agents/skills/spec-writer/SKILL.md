---
name: spec-writer
description: Generates strict software component specifications formatted as OKF-compliant Markdown files with YAML frontmatter.
---

# Spec-Driven Development (SDD) via OKF Protocol

You are in **Architect Mode**. Your goal is to define component contracts, props, and routing specs as valid **Open Knowledge Format (OKF)** Markdown documents.

Do NOT write implementation code or tests during this phase.

## Target Output Path
Save files to: `docs/specs/<feature-id>.md`

---

## OKF Specification Format

````markdown
---
type: specification
title: "Feature Name Specification"
description: "High-level overview of the feature boundary and contract."
tags:
  - "astro"
  - "component-spec"
generated:
  by: opencode/big-pickle
  at: "YYYY-MM-DDTHH:MM:SSZ"
status: draft
---

# Specification: Feature Name

## 1. Overview
Brief architectural description of the feature within the Astro application.

## 2. Component Structure & Hydration
- **Root Component:** `src/pages/route-name.astro`
- **Islands & Directives:**
  - `InteractiveWidget.tsx` -> `client:visible` (Hydrate only when scrolled into view)

## 3. Component Contracts (Props)

```typescript
export interface FeatureProps {
  /** Unique ID of the active user */
  userId: string;
  /** Initial state payload */
  initialData?: Record<string, unknown>;
}
```

## 4. Data Flow & Dependencies

- Data sourced from Astro frontmatter fetch (---).
- Implements contract from Global API Spec.
- Relies on User Auth Session.

## 5. Edge Cases & Errors

- Unhandled null states render <EmptyBanner/>.
- Network errors trigger client fallback.
````

## Instructions

1. **Analyze Requirements:** Map out Astro components, props, routing, and hydration strategies.
2. **Build YAML Frontmatter:** Set `type: specification`, `title`, `description`, `tags`, and `generated` details.
3. **Draft Markdown Body:** Provide clear TypeScript interfaces for `Props`, component trees, and hydration rules.
4. **Add OKF Graph Links:** Cross-link to dependent or related OKF concepts using bundle-relative paths (e.g., `[Auth Spec](/specs/auth.md)`).
5. **Save:** Output to `docs/specs/<feature-id>.md`.

## Hand-off Criteria
Pause and request user approval of `docs/s