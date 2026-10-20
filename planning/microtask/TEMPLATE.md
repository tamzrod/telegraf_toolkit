# TASK-ID — Concise outcome

> Template only. Files under Planning do not authorize execution.

## Intent
State one independently verifiable outcome and the relevant roadmap checkbox.

## Starting state
Cite specific repository source files and accepted architecture decisions. Identify what is implemented versus proposed.

## Required result
Describe observable artifacts without inventing unresolved design decisions.

## Execution boundary
- Allowed files/interfaces:
- Dependencies:
- Explicit non-goals:
- Privileged or destructive actions: none unless separately authorized.

## Acceptance criteria
- [ ] Specific observable result.
- [ ] Documentation and implementation agree.
- [ ] No new secrets, deployment or historian writes.

## Verification evidence
List the exact tests, checks and evidence to report; do not call skipped tests passing.

## Stop conditions
Stop for human decision if the task needs an unresolved ADR, new out-of-scope work, credentials or production access.

## Git lifecycle
Application code/scaffolding uses a task branch and human-reviewed PR, even while repository initialization is in progress. Archive the active task provisionally in the PR and confirm its removal from Active Work on `main` after merge.
