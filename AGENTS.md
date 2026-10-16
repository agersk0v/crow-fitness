# Instructions for coding agents

## Project context

Crow Fitness is a minimal workout tracker for one implicit user. The agreed stack is Next.js with TypeScript, a Go REST API, and PostgreSQL. Exercises belong to one internal local owner selected by Go (approved for #11). There is no authentication, login, account management, or multiple-user runtime support. Keep the application and its implementation simple.

The owner delegates implementation to agents and reviews their pull requests; do not require the owner to write code.

## Read before working

- Read the assigned issue, its acceptance criteria, and relevant discussion.
- Read `README.md` for the project overview and implemented setup instructions.
- Read `docs/PRODUCT.md` for authoritative current product behavior and separately identified future work.
- Read `docs/ARCHITECTURE.md` for the implemented technical design and API contracts. Follow approved product requirements if a technical proposal conflicts with them.
- Inspect the current repository and any applicable instructions before changing files. Documentation about future components does not mean those components exist.

Do not silently turn a recommendation or unresolved product choice into an approved requirement. If a decision is necessary to implement correct behavior, obtain the owner's decision before implementing that behavior; continue independent authorized work where possible.

## Branch and pull request workflow

1. Confirm the task's scope and inspect the current base branch and existing PRs to avoid duplicating work.
2. Create a focused feature branch from the intended base. Never commit or push directly to `main`.
3. Implement only the authorized issue scope. Feature-branch commits and pushes are allowed for that work. Preserve unrelated changes and avoid unnecessary refactoring or dependencies.
4. Update relevant documentation when behavior changes, within the issue's scope.
5. Review the diff and run appropriate available checks. Fix relevant failures and report any remaining blockers.
6. Open a PR targeting the intended base branch and link the issue, normally using `Closes #<number>` in the description.
7. Explain the resulting behavior, important decisions, validation, and any limitations. Leave the PR open for the owner's review and address requested revisions on its branch.

**Never merge PRs, enable auto-merge, or write directly to `main`. The owner handles merging.** Do not change repository permissions or branch-protection settings as part of ordinary implementation work.

These are agent working instructions, not a security boundary. GitHub permissions and repository rules must separately enforce any desired access restrictions.

## Implementation and data care

- Keep workout business rules in Go and persist records in PostgreSQL, following the reviewed design.
- Preserve recorded workout history when templates change. Keep suggested values separate from recorded sets.
- Keep secrets and machine-specific configuration out of Git.
- Use a separate test database; do not reset, overwrite, or delete the owner's workout data to validate a change.
- Add only the services and abstractions needed for the task. Do not add authentication or multiple-user features without a new approved requirement.

## Validation and documentation

Use the repository's actual tools and commands from README.md. PRODUCT.md owns current user-visible rules; ARCHITECTURE.md owns current technical contracts. Update those sections rather than appending issue-by-issue histories or duplicating them in the README. Keep proposals separate from implemented behavior; an open PR is not shipped.

Keep detailed validation and data-integrity checks in Go/PostgreSQL tests. The smoke script owns representative frontend-proxy integration, readiness recovery, and restart persistence on its disposable Compose project. Preserve meaningful coverage rather than removing checks just to reduce their count.

Do not invent setup, build, or test commands, or claim checks passed when they did not run. As tooling is implemented, document working commands in the README and use the relevant checks for each change.

For implementation work, verify the affected behavior and important error/data-integrity cases. For documentation-only work, check accuracy, links, examples, formatting, and file scope; application tests generally do not apply.

When handing off a PR, report what changed, why, what was checked, and any unresolved decision or blocker. Keep the explanation concise and understandable to the owner.
