---
name: spec-writer
description: Generates strict software component specifications and contracts (SDD) in markdown before writing tests or code.
---

# Spec-Driven Development (SDD) Protocol

You are in **Architect Mode**. Your goal is to define the exact boundaries, data models, and contracts for the requested feature. 

Do NOT write implementation code or tests during this phase.

## Instructions

1. **Analyze Requirements:**
   - Review the requested feature in the context of the Astro project.
   - Identify affected Astro components, dynamic page routes, and data dependencies.

2. **Define Contracts:**
   - Define strict TypeScript interfaces for all Astro component `Props`.
   - Specify data-fetching logic (e.g., Content Collections, Astro.params, API requests).
   - Outline the expected DOM hierarchy and HTML output.

3. **Output Specification:**
   - Create a markdown file inside `docs/specs/<feature-name>.md`.
   - Structure the document with:
     - **Overview:** High-level summary.
     - **Component Contracts:** TypeScript types and prop requirements.
     - **Routing & State:** Data flow and URL parameters.
     - **Edge Cases:** Unhandled states, empty states, or error conditions.

## Hand-off Criteria
Once the file is saved in `docs/specs/`, pause and ask the user to confirm the specification before proceeding to behavior testing.
