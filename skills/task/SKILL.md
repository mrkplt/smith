---
name: task
description: Manage Smith task contracts during loop execution with consistent lifecycle updates. Use when a loop is bound to a task_contract_id and needs start/log/handoff status updates.
---

# Task Workflow

Use this skill when a loop has `task_contract_id` metadata and must keep task state synchronized.

## Required Sequence

1. Start a task session at loop bootstrap:

```bash
task usage --new-session
```

2. Move the bound task into active execution:

```bash
task start <task-id>
```

3. Record meaningful lifecycle checkpoints:

```bash
task log <task-id> "<progress update>"
```

4. Write completion handoff details:

```bash
task handoff <task-id> --done a,b --remaining c,d
```

## Notes

- Keep logs milestone-based; avoid command-by-command spam.
- Keep `done` and `remaining` short, concrete, and outcome-focused.
- Do not mutate unrelated tasks.
