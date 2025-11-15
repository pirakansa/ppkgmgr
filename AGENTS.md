# AGENTS.md

This document is the **README for AI coding agents**. It complements the human-facing README.md so that agents can develop safely and efficiently.

---

## 1. Setup Steps

* Recommended: VS Code Dev Container / GitHub Codespaces (use the `.devcontainer/` image).
* Confirm you have write permission under `go env GOPATH` and that `go install` works.
* On the first run, execute `go mod tidy` at the project root to make sure dependencies are intact.
* Task runner: `Makefile` (CI uses the `make` targets described later).

---

## 2. Build & Run

* Build: `go build -o ./bin/host/ ./cmd/<name>`
* Test: `go test -v ./...`
* Run during development: `go run ./cmd/<name>`
* Cleanup: remove build artifacts (such as `./bin/`) with `rm -rf ./bin/` (equivalent to `make clean`).

---

## 3. Project Structure

We follow the **Standard Go Project Layout**.

```
.
├─ cmd/<name>/main.go     # CLI entry point
├─ internal/              # Non-exported packages
├─ pkg/                   # Reusable public logic
├─ test/                  # Test fixtures
├─ scripts/               # Installation helper scripts
├─ bin/                   # Build artifacts (generated; not tracked by Git)
├─ build/                 # Release artifacts
├─ Makefile               # Build / test / release tasks
└─ docs/                  # Documentation
```

### Roles and Guidelines

* Place shared logic in `internal/` or `pkg/`. Limit `main.go` under `cmd/` to CLI bootstrapping and argument handling.
* When adding a new CLI, create `cmd/<name>/main.go` and add a one-line description to both `README.md` and `AGENTS.md`.
* Put `_test.go` files in the same package directory as the code under test, and use fixtures under `test/` when needed.
* Keep public-facing logic in `pkg/` and internal-only logic in `internal/`, maintaining a one-way dependency direction.

### Agent-Specific Rules

* Place new files according to the directory guidelines above; avoid introducing unnecessary top-level directories.
* When modifying existing functions, add or update unit tests and confirm `go test ./...` passes.
* When writing files or accessing external resources, use temporary directories so existing test data is not overwritten.
* After any change, ALWAYS run `make build`. 

---

## 4. Coding Standards

* Always run `staticcheck ./...` so the code remains `staticcheck`-formatted.
* Run `go vet ./...` for static checks and ensure there are no warnings (CI requirement).
* Handle errors by returning `error`; do not silently discard them with `fmt.Println`. Prefer `fmt.Fprintf(os.Stderr, ...)` for user-facing messages.
* Package names must be lowercase words (no snake_case). Exported identifiers use UpperCamelCase.
* Extract magic numbers and hard-coded URLs into constants with meaningful names within the module.
* Avoid large, unrelated refactors and keep the impact of changes minimal.

---

## 5. Testing & Verification

* Unit tests: `go test ./...`
* Race detection: run `go test -race ./...` when concurrency issues are suspected.
* For additional file or network operations, use temp directories or `httptest` to avoid external dependencies.
* When command behavior changes, keep usage examples in `README.md` and fixtures under `test` consistent.

### Static Analysis / Lint / Vulnerability Scanning

* Static analysis: `go vet ./...`
* Code quality: `staticcheck ./...`
* Vulnerability scanning: `govulncheck ./...`

---

## 6. CI Requirements

GitHub Actions (`.github/workflows/go.yml`) runs the following:

* `make build`
* `make test`

Confirm `make build` / `make test` succeed locally before opening a PR. If they fail, format and validate locally, then rerun.

---

## 7. Security & Data Handling

* Do not commit secrets or confidential information.
* Do not log personal or authentication data in logs or error messages.
* Use fictitious URLs and passwords in test data; avoid hitting real services.
* Obtain user approval before accessing external networks (disabled by default in the agent environment).

---

## 8. Agent Notes

* If multiple `AGENTS.md` files exist, reference the one closest to your working directory (this repository only has the top-level file).
* When instructions conflict, prioritize explicit user prompts and clarify any uncertainties.
* Before and after your work, confirm `go build -o ./bin/host/ ./cmd/<name>` and `go test ./...` succeed. If they fail, report the cause and mitigation (the Makefile `build` / `test` targets wrap these commands).

---

## 9. Branch Workflow (GitHub Flow)

This project follows **GitHub Flow** based on `main`.

* **main branch**: Always releasable. Direct commits are forbidden; use pull requests.
* **Feature branches (`feature/<topic>`)**: Branch from `main` for new features or enhancements, then open a PR when done.
* **Hotfix branches (`hotfix/<issue>`)**: Branch from `main` for urgent fixes, merge promptly after CI passes.

### Rules

* Always branch from `main`.
* Assign reviewers when opening a PR and merge only after CI passes.
* Feel free to delete branches after merging.

---

## 10. Commit Message Policy

Commit messages follow **Conventional Commits**. Agents must comply. Write the comment section in **English**.

### Format

```
type(scope?): description
```

* `type`: feat / fix / docs / style / refactor / test / chore
* `scope`: Optional; module or directory names, etc.
* `description`: Describe the change concisely in English.

### Body

* Write the WHY (reason for the change) in a single English sentence.
* List the HOW (per-file changes) in English.

```
- internal/data/data.go: Added error return when YAML parsing fails
- pkg/req/req.go: Strengthened HTTP timeout configuration
```

### Granularity

* Default to one semantic change per commit.
* Separate generated code into logical units; do not mix with other changes.

### PRs and Commits

* Always document **Motivation / Design / Tests / Risks** in English in the PR description.
* Follow team policy on squashing after reviews; if none, keep the original commit structure.

---

## 11. Documentation Policy

* **README.md (top level)**:
  * Introduction: tool overview, usage, installation.
  * Later sections: developer build steps, testing instructions.
  * Keep it accessible so first-time users can onboard smoothly.

* **docs/**:
  * Create detailed designs or supplemental docs as needed. None exist yet, so define structure and filenames when adding.

* **Operational Guidelines**:
  * Update documentation alongside code changes; if none are needed, note "No documentation changes" in the PR description.
  * Verify sample code and command examples actually work.
  * Include generation scripts when submitting auto-generated docs.

---

## 12. Dependency Management Policy

* Add dependencies with `go get <module>@<version>` and keep `go.mod` / `go.sum` in sync.
* Remove unused dependencies with `go mod tidy`.
* For dependency updates, state the target module and reason in the PR body.
* Check external dependencies with `govulncheck ./...` and report as needed.

---

## 13. Release Process

* Follow **SemVer** for versioning.
* Tag new releases with `git tag vX.Y.Z` and verify `make release` outputs.
* Update CHANGELOG.md and reflect the changes in the release notes (include generators in the PR if they were used).

### 13.1 CHANGELOG.md Policy

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

## 14. PR Template

Include the following items when creating a PR:

* **Motivation**: Why this change is needed.
* **Design**: How you implemented it.
* **Tests**: Which tests were run.
* **Risks**: Potential side effects or concerns.

Template example:

```
### Motivation
...

### Design
...

### Tests
...

### Risks
...
```

---

## 15. Checklist

* [ ] `make lint`
* [ ] `make test`
* [ ] `make build`

