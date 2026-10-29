# UI-001 — Single-field inline editing (PPC/VRE control panel)

**HUMAN-AUTHORIZED CURRENT** on `phase-3/canonical-mapper`.

## Goal
In the pictured PPC/VRE control panel, each editable setpoint must use **one value box with two modes**, not stacked read and edit boxes.

## Scope and acceptance
- First locate the actual component for PPC/NGCP setpoints. Screenshot labels include `PPC Analog Command`, `NGCP Analog Command`, and `Active Power Setpoint`. If this component is **not in this repository/branch**, report `TASK_BLOCKED` identifying the correct repo/source needed; do not invent a substitute UI.
- **Read mode:** one box shows the persisted preview value. Clicking that same box enters edit mode in place, focuses/selects the input; no second box, extra save row or buttons.
- **Edit mode:** Enter validates and saves then returns to read mode; Escape cancels and restores the previous value. Invalid values must not overwrite the previous value. Preserve units (MW, MVAr, x1, kV) and existing values.
- Apply consistently to editable PPC/NGCP command setpoints; feedback measurements remain read-only. UI preview must not send live device commands.
- Add concise interaction tests for click→edit→Enter, Escape, invalid input, and feedback read-only. Run affected UI tests/build and repository checks; record real results.

## Checkpoint
Only after verified acceptance: archive this packet using canonical `archive/active-work/`, clear CURRENT in `handoff.md` and `workflow/active_work/README.md`, commit/push **normally**, and verify exact remote SHA. On missing source or failed checks retain CURRENT and report `TASK_BLOCKED`. No PR, merge, main modification, force-push, deployment, or historian/device writes.
