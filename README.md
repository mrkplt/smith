# Smith: A Distributed Runtime for Autonomous Software Development

**Turning product requirements into running software through autonomous development loops.**

Smith is a distributed runtime designed to execute autonomous development loops across a cluster of machines. By combining Product Requirement Documents (PRDs), GitHub Issues, and automated validation workflows, Smith enables software systems to continuously build themselves.

Instead of assigning tasks manually to engineers, Smith distributes autonomous "upstream tooling loops" across infrastructure. Each loop reads project requirements, evaluates the current repository state, implements tasks, validates changes, and contributes improvements back to the codebase.

The result is a new model of software development where repositories become active environments capable of continuously progressing toward their stated goals.

## The Problem

Modern software development is constrained by manual coordination. Even with modern tooling, teams still rely on humans to interpret PRDs, translate requirements into issues, implement code, validate results, and track progress. While automation has improved build pipelines and deployment, the development process itself remains largely manual.

At the same time, AI coding systems have emerged that can generate code, but they typically operate in isolated sessions without long-term memory or structured execution models. This creates a gap between AI capability and production software workflows.

## The Smith Approach

Smith bridges this gap by treating software development as a **continuous autonomous process**. Instead of orchestrating discrete workflows, Smith distributes persistent development loops across infrastructure. Each loop performs the following cycle:

1.  **Read** the project PRD
2.  **Evaluate** GitHub issues and repository state
3.  **Identify** the next unfinished task
4.  **Implement** the change
5.  **Run validation** (tests, builds, linting)
6.  **Commit** improvements
7.  **Repeat** until all requirements are satisfied

These loops run continuously across a cluster—typically Kubernetes—allowing development activity to scale horizontally.

## Key Concepts

### upstream tooling Loops

At the heart of Smith is the **upstream tooling Loop**, a structured feedback loop designed for reliable autonomous development. A upstream tooling loop externalizes state into the repository itself. Instead of relying on fragile model memory, it stores progress in Git commits, PRD documents, task tracking files, and automated tests.

Each iteration starts with fresh model context but reconstructs state from the repository, enabling long-running autonomous progress over thousands of iterations.

### Choreography, Not Orchestration

Traditional workflow engines rely on centralized orchestration. Smith takes a different approach: **Choreography**.

There is no central controller dictating every step. Instead:
- **PRDs** define goals.
- **GitHub issues** define tasks.
- **Tests** define correctness.
- **Agents** react to repository state.

The repository becomes the shared coordination layer where autonomous loops cooperate. This allows Smith to support large numbers of development loops working simultaneously across multiple repositories.

## Purpose

Smith coordinates these autonomous execution loops as a state machine stored in etcd. It is designed to:

- accept operator ingress requests (direct, GitHub issues, PRD tasks),
- convert each request into a deterministic loop lifecycle (`unresolved -> running -> synced|flatline|cancelled`),
- enforce safe concurrency with per-loop locks and revision-checked state transitions,
- run loop workers as Kubernetes Jobs and preserve execution evidence (journal, handoff, override, audit).

In this repository, the focus is the MVP control plane, deployment assets, and verification/test harnesses.

## Philosophy

Smith intentionally does not personify agents.

- **Homogeneous Workers:** Agents are modeled as homogeneous and omnicapable execution units, not distinct personalities. Anthropomorphizing agents is treated as an implementation constraint that reduces operational flexibility and performance.
- **Uniform Replication:** The target model is many equivalent workers, same contract, same capabilities, horizontally scalable.
- **Role-Neutral Primitives:** The system design favors state, locks, jobs, and handoffs over persona-specific behavior.
- **Distributed Substrate:** Smith moves beyond a single-machine file-system model by using etcd + Kubernetes as the control substrate, so execution can scale across distributed compute while preserving deterministic state and traceability.

The platform direction is informed by upstream tooling, [marcus/sidecar](https://github.com/marcus/sidecar), [marcus/td](https://github.com/marcus/td), and related projects, but is engineered to scale beyond a single developer machine.

## Installation and Usage

`smithctl` is the primary CLI for interacting with Smith. Installation, usage examples, and operator workflows are documented in [docs/smithctl-installation-and-usage.md](docs/smithctl-installation-and-usage.md).

## Architecture Summary

Smith is split into control-plane and data-plane components.

### Control Plane

- `smith-api` (`cmd/smith-api`): HTTP API for loop create/list/get, GitHub + PRD ingress, operator override actions, provider auth lifecycle, and cost reporting. Includes Swagger UI at `/swagger/` and gRPC support on port 8081.
- `smith-chat` (`cmd/smith-chat`): dedicated chat service for session creation, message streaming (SSE), and chat action commit forwarding.
- `smith-core` (`cmd/smith-core`): watches unresolved loop state in etcd, acquires per-loop locks, transitions loop state, and schedules replica Jobs in Kubernetes.
- `smith-daemon` (`cmd/smith-daemon`): background control-plane worker for retention-based cleanup of terminal loops and runtime artifact pruning.
- `smithctl` (`cmd/smithctl`): kubectl-style operator CLI for `loop` and `prd` resources with context/config support and scriptable JSON output.
- `smith` (`cmd/smith`): PRD launcher CLI (`smith --prd`) for interactive PRD generation before build loops.
- `smith-mcp` (`cmd/smith-mcp`): Model Context Protocol (MCP) server for tool-based AI integration.
- `smith-console` (`console/` + Helm deployment): operator UI/runtime assets.
- `pkg/client/v1`: Typed Go client library for Smith API and chat endpoints.
- etcd: authoritative source of truth for anomalies, loop lifecycle state, locks, journal events, handoffs, overrides, and audit records.

### Data Plane

- `smith-replica` (`cmd/smith-replica`): Kubernetes Job worker that executes loop work, appends journal entries, writes handoff output, and finalizes loop state.

### Deployment and Ops Assets

- Helm chart: `helm/smith`
- Dockerfiles: `docker/`
- Core implementation: `internal/source/`
- Supporting docs: `docs/`
- Make-first local workflow: `make help` (doctor/bootstrap/cluster/deploy/test/teardown)

API contracts, ingress flows, terminal control details, and contributor workflows are documented in:

- [docs/loop-ingress-and-cli.md](docs/loop-ingress-and-cli.md)
- [docs/loop-retention-cleanup.md](docs/loop-retention-cleanup.md)
- [docs/task-contracts-and-feature-capability.md](docs/task-contracts-and-feature-capability.md)
- [docs/feature-flags.md](docs/feature-flags.md)
- [docs/environment-variables.md](docs/environment-variables.md)
- [docs/documentation-audit-chat-wip.md](docs/documentation-audit-chat-wip.md)
- [docs/contributing.md](docs/contributing.md)
- [docs/local-dev-make-workflow.md](docs/local-dev-make-workflow.md)
- [docs/index.md](docs/index.md)

## Technology Stack and Thanks

See the dedicated documentation page for:
- the current technology stack (Kubernetes, Helm, vCluster, etcd, Go, Docker, and related tooling),
- acknowledgments and inspiration credits, including [marcus/td](https://github.com/marcus/td), [marcus/sidecar](https://github.com/marcus/sidecar), and upstream tooling.

Reference: [docs/technology-stack-and-thanks.md](docs/technology-stack-and-thanks.md)

## PRD-First Loop Workflow

Generate a PRD JSON:

```bash
smith --prd "Build issue-driven loop execution with terminal attach support" --out .agents/tasks/prd.json
```

If a PRD already exists at `.agents/tasks/prd.json`, replica issue/prompt workflows skip PRD generation and move straight to iterative build.

For canonical markdown import, JSON validation, markdown export, and ingress workflow details, see [docs/prd-authoring-workflow.md](docs/prd-authoring-workflow.md).

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
