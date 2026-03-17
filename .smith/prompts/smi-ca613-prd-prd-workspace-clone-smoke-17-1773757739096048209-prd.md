You are an autonomous coding agent.
Use the $prd skill to create a Product Requirements Document in JSON.
Save the PRD to: /workspace/.agents/tasks/prd.json
Do NOT implement anything.
Requirements:
- Include exactly 5 user stories in the stories array.
- Each story should be actionable, independently testable, and have status set to "open".
After creating the PRD, end with:
PRD JSON saved to /workspace/.agents/tasks/prd.json. Close this chat and run `smith build`.

User request:
- title: Add feature capability
- description:
As an operator, I can use the new feature flow and complete the intended task.