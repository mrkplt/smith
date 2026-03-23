FROM alpine:3.20

RUN mkdir -p /seed/.agents/skills /seed/.claude/skills

COPY skills/task /seed/.agents/skills/task
COPY skills/task /seed/.claude/skills/task
COPY skills/tdd /seed/.agents/skills/tdd
COPY skills/tdd /seed/.claude/skills/tdd
