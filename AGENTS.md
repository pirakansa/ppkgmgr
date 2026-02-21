# AGENTS.md

This document is the **README for AI coding agents**. It complements the human-facing README.md so that agents can develop safely and efficiently.

---


## Documentation of Process vs Policy

This repository separates **policy** from **how-to guidance**:

- **AGENTS.md = Policy (MUST/MUST NOT)**  
  Contains the mandatory rules agents must follow (e.g., language requirements, required sections, validation expectations, boundaries).
  Keep it short and stable.

- **SKILLS = Procedure / Templates / Checklists**  
  Contains step-by-step workflows, templates, and checklists used to comply with policy.
  Prefer updating skills when improving writing structure or workflow details.

Rule of thumb:
- If it is a non-negotiable rule for reviews/CI: put it in **AGENTS.md**.
- If it is an example, template, or writing process: put it in a **skill**.


---

## Setup Steps

* Confirm you have write permission under `go env GOPATH` and that `go install` works.
* On the first run, execute `go mod tidy` at the project root to make sure dependencies are intact.
* Task runner: `Makefile` (CI uses the `make` targets described later).

---

## Build & Validate

* Build: `make build`
* Test: `make test`
* Lint: `make lint`
* Cleanup: remove build artifacts (such as `./bin/`) with `rm -rf ./bin/` (equivalent to `make clean`).
* For CLI usage and command examples, see the Usage section in README.md.

---

## Project Structure

We follow the **Standard Go Project Layout**.

```
.
├─ cmd/<name>/main.go     # CLI entry point
├─ internal/              # Non-exported packages
├─ pkg/                   # Reusable public logic
├─ test/                  # Test fixtures
├─ bin/                   # Build artifacts (generated; not tracked by Git)
├─ Makefile               # Build / test / release tasks
└─ docs/                  # Documentation
```

### Roles and Guidelines

* Place shared logic in `internal/` or `pkg/`. Limit `main.go` under `cmd/` to CLI bootstrapping and argument handling.
* When adding a new CLI, create `cmd/<name>/main.go` and add a one-line description to both `README.md` and `AGENTS.md`.
* Put `_test.go` files in the same package directory as the code under test, and use fixtures under `test/` when needed.
* Keep public-facing logic in `pkg/` and internal-only logic in `internal/`, maintaining a one-way dependency direction.
* For CLI work, keep to the following subpackage layout:
  * `internal/cli/commands/<name>` – Cobra command definitions only; call into shared helpers instead of embedding logic here.
  * `internal/cli/manifest` – Utilities dedicated to manifest handling (download orchestration, target resolution, integrity checks, etc.).
  * `internal/cli/shared` – Types and helpers used across the CLI (`DownloadFunc`, error wrappers, path helpers, digest helpers, …).
  * Enforce the one-way dependency `commands -> manifest/shared`; no other package should import from `internal/cli/commands`.

### Agent-Specific Rules

* Place new files according to the directory guidelines above; avoid introducing unnecessary top-level directories.
* When modifying existing functions, add or update unit tests and confirm `make test` passes.
* When writing files or accessing external resources, use temporary directories so existing test data is not overwritten.


---

## Coding Standards

* Always run `make staticcheck` so the code remains `staticcheck`-formatted.
* Run `make lint` for static checks and ensure there are no warnings (CI requirement).
* Handle errors by returning `error`; do not silently discard them with `fmt.Println`. Prefer `fmt.Fprintf(os.Stderr, ...)` for user-facing messages.
* Package names must be lowercase words (no snake_case). Exported identifiers use UpperCamelCase.
* Extract magic numbers and hard-coded URLs into constants with meaningful names within the module.
* Avoid large, unrelated refactors and keep the impact of changes minimal.

---

## Testing & Verification

* Unit tests: `make test`
* For additional file or network operations, use temp directories or `httptest` to avoid external dependencies.
* When command behavior changes, keep usage examples in `README.md` and fixtures under `test` consistent.

### Static Analysis / Lint / Vulnerability Scanning

* Static analysis: `make staticcheck`
* Code quality: `make vet`
* Vulnerability scanning: `make govulncheck`

---

## CI Requirements

GitHub Actions (`.github/workflows/go.yml`) runs the following:

* `make lint`
* `make test`
* `make build`

Confirm `make lint` / `make test` / `make build` succeed locally before opening a PR. If they fail, format and validate locally, then rerun.

---

## Security & Data Handling

* Do not commit secrets or confidential information.
* Do not log personal or authentication data in logs or error messages.
* Use fictitious URLs and passwords in test data; avoid hitting real services.
* Obtain user approval before accessing external networks (disabled by default in the agent environment).

---

## Agent Notes

* If multiple `AGENTS.md` files exist, reference the one closest to your working directory (this repository only has the top-level file).
* When instructions conflict, prioritize explicit user prompts and clarify any uncertainties.
* Before and after your work, ensure `make lint`, `make test`, and `make build` all succeed; report the cause and fix if any of them fail.


---

## Branch Workflow (GitHub Flow)

This project follows **GitHub Flow** based on `main`.

* **main branch**: Always releasable. Direct commits are forbidden; use pull requests.
* **Feature branches (`feature/<topic>`)**: Branch from `main` for new features or enhancements, then open a PR when done.
* **Hotfix branches (`hotfix/<issue>`)**: Branch from `main` for urgent fixes, merge promptly after CI passes.

### Rules

* Always branch from `main`.
* Assign reviewers when opening a PR and merge only after CI passes.
* Feel free to delete branches after merging.

---

## Commit Message Policy

Commit messages MUST follow **Conventional Commits** and MUST be written in **English**.

### Header
`type(scope?): description`

- `type`: feat / fix / docs / style / refactor / test / chore
- `scope`: optional (module/package/directory)
- `description`: concise present-tense English summary

### Body
- First body line MUST state the **WHY** (reason for the change) in a single English sentence.
- Then list the **HOW** as per-file bullet points in English (`path: concrete change`).
- Do not claim tests passed unless they were actually run.

### Granularity
- One semantic change per commit.
- Keep generated files separate when practical; do not mix with other changes.

For structured authoring (template, checklist), use the skill: `conventional-commits-authoring`.

---

## Documentation Policy

- **Language**: All documentation (README.md, docs/, inline doc-comments) MUST be written in **English**.
- **README.md (top level)** is onboarding-first: overview, install, and one quick-start. Keep it short and link to details in `docs/`.
- **docs/** holds detailed documentation and is organized as:
  - **User guides** (practical usage / workflows)
  - **Specification references** (contracts: schema, flags, processing rules)
  - If content mixes both, split it into the appropriate documents.
- **Source of truth**
  - For post-implementation updates, treat **code + passing tests** as SoT and use `docs-maintenance-implementation-sync`.
  - For design-first work where the **spec is SoT**, use the spec-driven skills (`spec-driven-doc-authoring` / `spec-to-code-implementation`).
- **PR hygiene**: Update docs with behavior changes. If no doc updates are needed, explicitly note **"No documentation changes"** in the PR description.
---

## Dependency Management Policy

* Add dependencies with `go get <module>@<version>` and keep `go.mod` / `go.sum` in sync.
* Remove unused dependencies with `go mod tidy`.
* For dependency updates, state the target module and reason in the PR body.
* Check external dependencies with `make govulncheck` and report as needed.

---

## Release Process

* Follow **SemVer** for versioning.
* Tag new releases with `git tag vX.Y.Z` and verify `make release` outputs.
* Update CHANGELOG.md and reflect the changes in the release notes (include generators in the PR if they were used).

### CHANGELOG.md Policy

* **Sections**: Follow `[Keep a Changelog]` categories - `Added / Changed / Fixed / Deprecated / Removed / Security`.
* **Language**: English.
* **Writing Principles**:
  * Describe "what changes for the user" in one sentence; include implementation details only when needed.
  * Emphasize **breaking changes** in bold and provide migration steps.
  * Include PR/Issue numbers when possible (e.g., `(#123)`).
* **Workflow**:
  1. Add entries to the `Unreleased` section in feature PRs.
  2. Update the version number and date in release PRs.
  3. After tagging, copy the relevant section into the release notes.
* **Links (recommended)**:
  * Add comparison links at the end of the file.
* **Supporting Tools** (optional):
  * Use tools like `git-cliff` or `conventional-changelog` to draft entries, then edit manually.

---

## PR Template

PR descriptions MUST be written in **English** and MUST include:
- Motivation
- Design
- Tests (only what was actually run)
- Risks

For structured authoring (template, checklist), use the skill: `pr-description-authoring`.

---

## Checklist

* [ ] `make lint`
* [ ] `make test`
* [ ] `make build`
