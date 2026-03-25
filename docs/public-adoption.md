# Public Adoption Playbook

This is the execution checklist for the roadmap item: "use `qp.yaml` in other public projects."

## Goal

Use `qp` visibly in public repositories so people can see practical examples without reading long docs first.

## Candidate Repos

- repos with active commits and existing `Makefile` / `package.json` / `justfile`
- repos where `test`, `build`, and `check` are already stable
- repos with CI where `qp guard` can run in pull requests

## Rollout Steps Per Repo

1. Add `qp` with `qp init --from-repo --docs`.
2. Trim and review generated tasks/scopes.
3. Ensure a passing `check` task and a useful default task.
4. Add `qp guard` for local and CI verification.
5. Commit `qp.yaml`, `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md`.
6. Update the repo README with a minimal usage block:
   - `qp list`
   - `qp check`
   - `qp guard`

## Quality Bar

- `qp list` is understandable without prior project knowledge.
- `qp check` passes from a clean checkout.
- `qp guard` provides a reliable pre-PR quality gate.
- scoped tasks exist for major code areas (to support agent workflows).

## Suggested Tracking

Track adopted repos in this file once complete:

- `<repo-name>`: `<date>` (link to PR/commit)
