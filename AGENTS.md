# Telegraf Toolkit — Agent Router

Short, always-on entry point for OpenHands and other development agents; not a project diary.

## Authority and commands
- `IDENTITY.md` defines product scope and accepted constraints; relevant accepted ADRs and `docs/` specifications control their domains. Report conflicts for human resolution.
- Git source is authoritative; ICC is a context cache, not execution authority. Never describe mocks or planned behavior as implemented.
- `Operation CWAL`: read `operation cwal.md` and follow its bounded execution rules.
- `BLACK SHEEP WALL`: read `BLACK_SHEEP_WALL.md` and load minimum relevant ICC context.
- Planning does not authorize implementation. Only human-promoted tasks in `workflow/active_work/` do.

## Safety invariants
- Official Telegraf 1.40.1 initial baseline; no maintained fork, floating dependencies or unverified production image digests. Pin exact versions and lockfiles.
- Keep independent runtimes and historical sync lifecycles; no automatic deploy, backfill or writes.
- Equal point counts do not prove integrity; preserve timestamps, series tags and field types.
- Keep credentials server-side; never commit, expose, log or export secrets or grant browser Docker control.
- Production changes, historian writes/deletes and credential operations require separate explicit authorization.

## Git policy
- `handoff.md` declares the human-owned workflow phase: `initialization` or `development`.
- **Initialization exception:** only explicitly authorized repository setup, agent workflow, governance and foundation documentation may be committed directly to `main`. This exception does not cover ordinary application implementation, even while initialization is underway.
- **Development (default):** code, features, tests, fixes and application scaffolding use a dedicated task branch and PR to `main`, with required checks and human approval before merge.
- If phase/task classification is missing or ambiguous, use the branch/PR workflow. Agents must not self-extend the initialization exception or switch phases.
- Only one CWAL writer may operate on the same task/work surface; during initialization, only one direct-`main` writer. No force pushes or overwriting concurrent changes.

## Operating posture
- Execute only the current authorized task. Prefer coherent, testable changes; never modify unrelated work.
- Report actual verification outcomes, limitations and blockers. Archive only verified work and confirm the final remote checkpoint under `operation cwal.md`.
