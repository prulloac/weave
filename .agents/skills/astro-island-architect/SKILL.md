---
name: astro-island-architect
description: Audits UI interactive components in Astro and determines the correct hydration strategy (client:visible, client:idle, client:media) or static rendering.
---

# Astro Island & Hydration Protocol

When introducing UI framework components (React, Vue, Svelte) or interactive islands:

1. **Default to Zero JS:** Always check if the UI can be rendered as a pure `.astro` component first.
2. **Select Hydration Directives Carefully:**
   - **No directive:** Pure static HTML (no JS sent to client).
   - `client:visible`: For below-the-fold interactive components (e.g., dynamic widgets, comment sections).
   - `client:idle`: For low-priority interactive features (e.g., chat overlays, analytics).
   - `client:media="(query)"`: For mobile menus or responsive-only interactivity.
   - `client:load`: **STRICTLY RESERVED** for critical above-the-fold dynamic UI (e.g., immediate user auth bar).
3. **Prevent Waterfalls:** Never fetch data inside client-side components if it can be fetched in the Astro frontmatter (`---`) and passed via props.
