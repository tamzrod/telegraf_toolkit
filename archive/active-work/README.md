# Verified Task Archive

A task is durably complete only when `origin/main` contains its verified implementation/evidence and moved task file, with the file absent from `workflow/active_work/`. A move on an unmerged PR branch is provisional. During narrowly authorized initialization, commit/push the verified archive move directly to `main`; during regular development use a task branch, required checks, human-reviewed PR and confirmed merge. Archived tasks never grant authorization.
