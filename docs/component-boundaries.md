# Component Boundaries: `shared` vs `features/*`

## Core Rule
Share by behavior, not by looks.

A component belongs in `shared` only when behavior is stable, reused, and context-independent.

## Placement Rules
1. Keep in `features/*` if it depends on feature-specific routes, query params, view models, or domain semantics.
2. Move to `shared` only when at least two real features use the same behavior with light configuration.
3. Do not extract for speculative reuse.
4. Keep in feature if extraction introduces many flags/branches to support feature differences.
5. Prefer `shared` for stable primitives and infrastructure components (for example: icons, buttons, paging, generic table/map switching).

## Decision Checklist
Promote to `shared` only if all are true:
1. Name can be domain-agnostic (no `sensor`, `thing`, `alarm`, `tenant`, etc.).
2. API can use generic props/slots, not feature-specific model types.
3. Behavior is identical across call sites.
4. Public API remains small and predictable.

If any item is false, keep it in the feature package.

## Promotion Workflow
1. Build in the feature package first.
2. If another feature needs it, allow short-term duplication while behavior stabilizes.
3. On second concrete reuse, extract to `shared` with a minimal API.
4. Replace duplicates and keep one source of truth.

## Red Flags
1. A `shared` component imports a feature package.
2. `shared` API includes feature/domain terms.
3. `shared` component has switch/if branches per feature.
4. Reuse requires passing full feature view models.

## PR Check
1. For each new component, state why placement is `shared` or `features/*`.
2. If `shared`, list at least two real call sites (unless it is a foundational primitive).
