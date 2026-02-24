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
1. Main template name matches file name.
2. Grouped files use family naming (for example `button.templ`, `button_helpers.templ`).

## Boundary Rule
1. `shared` stays domain-agnostic.
2. `shared` must not import from `features/*`.

## PR Check
1. If a file has multiple templates, explain in one line why they are grouped.
