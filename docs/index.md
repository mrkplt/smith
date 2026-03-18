# Smith Documentation

**A Distributed Runtime for Autonomous Software Development**

Smith is a distributed runtime designed to execute autonomous development loops across a cluster of machines. By combining Product Requirement Documents (PRDs), GitHub Issues, and automated validation workflows, Smith enables software systems to continuously build themselves.

## Core Concepts

- **Anomaly:** A discrete unit of work, bug, or task. In the Smith metaphor, an anomaly represents a deviation from the desired state of the system that must be resolved.
- **upstream tooling Loops:** Structured feedback loops that externalize state into the repository, enabling long-running autonomous progress.
- **Choreography:** A decentralized approach to development where PRDs define goals and agents react to repository state.
- **Continuous Implementation:** A shift from traditional CI/CD pipelines to an autonomous `plan -> implement -> validate -> iterate` loop.

## Philosophy

- **No Personified Agents:** Execution units are homogeneous, omnicapable, and designed for uniform horizontal replication.
- **Scale Beyond the Local Machine:** Using etcd + Kubernetes as the control substrate allows development activity to scale across distributed compute while preserving deterministic state and traceability.
- **Repository-Based Coordination:** The repository is the shared coordination layer where autonomous loops cooperate.
- **Inspiration:** The design is influenced by upstream tooling, [marcus/sidecar](https://github.com/marcus/sidecar), [marcus/td](https://github.com/marcus/td), and related work.

## Getting Started

- [Distributed Autonomous Orchestration MVP](distributed-autonomous-orchestration-mvp.md)
- [Contributing to Smith](contributing.md)
- [MVP Boundary and Release Gates](mvp-boundary-and-release-gates.md)
- [Pre-Release System Gate](pre-release-system-gate.md)
- [Local Make Quickstart](make-local-quickstart.md)
- [Local Development Reference](local-dev-make-workflow.md)

## Contributor Guide

- [Contributing to Smith](contributing.md)
- [Local Development Reference](local-dev-make-workflow.md)
- [Local Integration Environment](local-integration-environment.md)
- [smithctl Installation and Usage](smithctl-installation-and-usage.md)
- [Git History Policy](git-history-policy.md)
- Required repo `mise.toml` runtime bootstrap is documented in the contributor guide.

## Architecture

- [Distributed Autonomous Orchestration MVP](distributed-autonomous-orchestration-mvp.md)
- [PRD1 - Core Architecture](prd1.md)
- [Technology Stack and Thanks](technology-stack-and-thanks.md)
- [ADR 0001 - etcd State Machine with Kubernetes Jobs](adrs/0001-etcd-state-machine-and-kubernetes-jobs.md)
- [ADR 0002 - Single-Writer Locking and Reconciliation](adrs/0002-single-writer-locking-and-reconciliation.md)
- [ADR 0003 - Multi-Ingress and Environment Contract](adrs/0003-multi-ingress-and-environment-contract.md)
- [ADR 0004 - Provider Registry and Adapter Interface](adrs/0004-provider-registry-and-adapter-interface.md)
- [ADR 0005 - Completion Saga for Code and State Sync](adrs/0005-completion-saga-for-code-and-state-sync.md)
- [ADR 0006 - Provider Credentials in Kubernetes Secrets](adrs/0006-provider-credentials-in-kubernetes-secrets.md)
- [ADR 0007 - Dedicated Chat Service Boundary](adrs/0007-dedicated-chat-service-boundary.md)
- [ADR 0008 - smithctl as Primary Operator Interface](adrs/0008-smithctl-as-primary-operator-interface.md)
- [ADR 0009 - Contract-First API, gRPC, Client, and MCP](adrs/0009-contract-first-api-grpc-client-and-mcp.md)
- [ADR 0010 - Helm Chart as Deployment Contract](adrs/0010-helm-chart-as-deployment-contract.md)
- [ADR 0011 - Mount PRD into Runtime via ConfigMap](adrs/0011-mount-prd-into-runtime-via-configmap.md)
- [ADR 0012 - Provider-Specific Runtime Invocation](adrs/0012-provider-specific-runtime-invocation.md)
- [ADR 0013 - Migrate Operator Frontend to Svelte 5](adrs/0013-migrate-operator-frontend-to-svelte5.md)
- [ADR 0014 - Stable Short-Hash ID Generation](adrs/0014-stable-short-hash-id-generation.md)
- [ADR 0015 - Local CI Parity via act](adrs/0015-local-ci-parity-via-act.md)
- [ADR 0016 - PRD Readiness as Ingress Gate](adrs/0016-prd-readiness-as-ingress-gate.md)
- [ADR 0017 - Standardize CI Runtimes and Docs Build Path](adrs/0017-standardize-ci-runtimes-and-docs-build-path.md)
- [ADR 0018 - Retention Cleanup via smith-daemon with ConfigMap Policy Reload](adrs/0018-daemon-retention-cleanup-controller.md)
- [ADR 0019 - Task-Contract-Gated Loop Execution and Status Synchronization](adrs/0019-task-contract-gated-loop-execution.md)
- [etcd Key Schema](etcd-key-schema.md)
- [Reconciliation Loop](reconciliation-loop.md)
- [Completion Commit Protocol](completion-commit-protocol.md)
- [Lock Strategy](lock-strategy.md)

## Requirements and Traceability

- [Requirements (FR/NFR)](requirements-fr-nfr.md)
- [Requirements Traceability](requirements-traceability.md)

## Deployment

- [Deployment Recommendations](deployment-recommendations.md)
- [Kubernetes Secrets Encryption Provider Runbook](kubernetes-secrets-encryption-provider-runbook.md)
- [Cluster Autoscaler Prerequisites and Runbook](cluster-autoscaler-prerequisites-runbook.md)
- [Image Tagging and Versioning](image-tagging-versioning.md)
- [Helm Upgrade/Rollback Runbook](helm-upgrade-rollback-runbook.md)
- [Docs Site: Zensical + GitHub Pages](docs-site-github-pages.md)
- [Docs Site Style Contract (Sidecar-Inspired)](docs-site-style-sidecar.md)

## Agent Providers

- [Agent Provider Authentication](agent-provider-auth.md)

## Operations Notes

- [Changelog](changelog.md)
- [Documentation Audit (chat-wip)](documentation-audit-chat-wip.md)
- [Docs-to-PRD Lifecycle](docs-to-prd-lifecycle.md)
- [PRD Authoring Workflow](prd-authoring-workflow.md)
- [PRD Authoring Example](examples/prd-authoring/valid-prd.md)
- [PRD Implementation Status](prd-implementation-status.md)
- [Loop Ingress and CLI](loop-ingress-and-cli.md)
- [Settings API and Console Configuration](settings-api-and-console.md)
- [Loop Retention and Cleanup](loop-retention-cleanup.md)
- [Task Contracts and Feature Capability Flow](task-contracts-and-feature-capability.md)
- [Feature Flags Map](feature-flags.md)
- [Environment Variables (Important)](environment-variables.md)
- [Loop Environment Profile](loop-environment-profiles.md)
- [Multi-Provider Skill Mount Abstraction](multi-provider-skill-mount-abstraction.md)
- [Skill Volume Mounts for Loop Runtime](skill-volume-mounts.md)
- [Repository Auth Options](repository-auth-options.md)
- [Journal Retention and Archival Policy](journal-retention-archival-policy.md)
- [Observability Latency Validation](observability-latency-validation.md)
- [Backup/Restore Disaster Recovery Runbook](backup-restore-dr-runbook.md)
- [Staging Soak/Chaos Runbook](staging-soak-chaos-runbook.md)
- [Test Matrix and Failure Injection](test-matrix-and-failure-injection.md)
- [Go-Native Test Harness Strategy](test-harness-strategy.md)
