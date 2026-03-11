# Template File Splitting Guide

## Rule of Thumb
- `shared`: strict.
- `features/*`: pragmatic.

## Shared (`components/shared`)
1. Default: one `templ` per file.
2. Allowed grouping only for:
   - tightly related variants (same component family),
   - tiny construction helpers for that family.
3. Never group unrelated templates in `shared`.

## Features (`components/features/*`)
1. Multiple templates in one file are allowed when they form one page flow and change together.
2. Split into separate files when any of these is true:
   - template is reused independently,
   - file becomes hard to scan,
   - different owners usually touch different parts.

## Naming
1. Prefer descriptive role names, not generic `Component`:
   - `*View`, `*Edit`, `*List`, `*Filter`, `*Table`, `*Map`, `*Modal`, `*Actions`.
2. Helper/template file suffixes are explicit:
   - `_helpers.templ` for helper templates/functions tied to one feature flow.
   - `_properties.templ` for domain property rendering blocks.
   - `_graph.templ` for chart/graph templates and graph helper functions.
   - `_edit.templ` for edit form composition.
3. Main template name does not have to match file name.
4. `shared` templates stay noun/primitive-based (for example `button.templ`, `table.templ`, `common.templ`), not feature-flow names.
5. Avoid vague names like `common2`, `misc`, `temp`, or `component`.

## Boundary Rule
1. `shared` stays domain-agnostic.
2. `shared` must not import from `features/*`.

## PR Check
1. If a file has multiple templates, explain in one line why they are grouped.
