---
name: refactor-structural-readability
description: Use this skill when refactoring existing Go code for readability and maintainability without changing behavior, especially when files are large, responsibilities are mixed, naming is inconsistent, or logic is duplicated. Do not use this skill for feature development, architecture redesign, or behavior-changing refactors.
---

# Refactor Structural Readability (Go)

## Goal
Improve readability and maintainability while preserving behavior.

## Inputs
- Target files/packages
- Current constraints (public API stability, compatibility, coding style)
- Validation commands (`make test`, `make lint`, `make build`)

## Outputs
- Smaller responsibility-focused files/functions
- Consistent meaning-based naming
- Centralized duplicated logic (single source of truth)
- Reduced unnecessary public surface area
- Passing tests/lint/build

## Workflow
1. Baseline first
   - Inspect file lengths and responsibility concentration.
   - Identify duplicated conditions, deep nesting, and unclear naming.
2. Decompose by responsibility
   - Keep public entrypoints thin.
   - Split internal logic into focused units (flow vs helpers).
   - Prefer one file per concern when practical.
3. Normalize naming
   - Replace abbreviations with meaning-based names.
   - Use consistent verb+noun function names.
   - Use path/intent-aware local variable names.
4. Centralize duplicated logic
   - Move shared predicates/rules into one package-level source.
   - Call shared logic from consumers; avoid re-implementations.
5. Tighten API boundaries
   - Keep internals unexported unless needed externally.
   - Expose one high-level API when callers should not know implementation details.
6. Validate continuously
   - Format modified files.
   - Run tests, lint, build.
   - Fix only issues related to refactor scope.

## Safety Rules
- Do not change behavior intentionally.
- Do not mix feature additions with refactoring.
- Do not perform broad unrelated rewrites.
- Preserve error semantics, CLI/user-visible behavior, and compatibility expectations.

## Done Criteria
- Main flow can be read top-down without jumping excessively.
- Duplicated decision logic is consolidated.
- Naming is consistent and self-explanatory.
- Public API surface is minimal and intentional.
- `make test && make lint && make build` succeeds.

## Quick Checklist
- [ ] Responsibility split applied
- [ ] Naming normalized
- [ ] Duplicate logic centralized
- [ ] API boundary reviewed
- [ ] Validation commands passed
