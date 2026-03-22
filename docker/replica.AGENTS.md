## MANDATORY: Use task for Task Management

You must run `task usage --new-session` at conversation start (or after `/clear`) to see current work.
Use `task usage -q` for subsequent reads.

## Task Workflow
- Use `task start <task-id>` when beginning implementation.
- Record progress with `task log <task-id> <message>`.
- Submit handoff details with `task handoff <task-id> --done ... --remaining ...`.
