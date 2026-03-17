---
id: smith-cli-architecture-prd
title: Smith CLI Architecture PRD
status: approved
doc_type: prd
source_doc: docs/planning/approved/smith-cli-architecture.md
target_branch: chat-wip
owner: lars
---

# Product Requirements Document (PRD)

# Smith CLI Architecture (smithctl)

## Overview

`smithctl` is the command-line interface for interacting with Smith. The current CLI was originally designed around loop ingress and runtime execution. As Smith evolves into a developer-owned control plane with durable configuration objects (Provider Profiles, Projects, Integrations, Secrets), the CLI must evolve to reflect the same architecture.

This PRD proposes restructuring `smithctl` to align with the new Smith platform model where:

- Durable configuration objects are created and managed first
- Runtime execution references those configuration objects
- Authentication and external integrations are handled separately
- The CLI mirrors the structure of the Smith UI

The result is a CLI that manages **installation configuration first and runtime actions second**.

---

# Goals

1. Align CLI architecture with the new Smith configuration model.
2. Introduce first-class commands for Providers, Projects, Secrets, and Integrations.
3. Reduce configuration complexity in loop creation commands.
4. Replace inline configuration flags with references to durable configuration objects.
5. Maintain a clean and consistent CLI grammar.
6. Preserve backwards compatibility during migration.

---

# Non-Goals

- Removing existing loop functionality
- Breaking existing CLI workflows immediately
- Introducing multi-user permission systems

---

# CLI Philosophy

The CLI should reflect Smith's identity as a **runtime control plane**.

Two distinct categories of commands should exist:

## Configuration Plane

Commands that manage durable system configuration.

Examples:

- providers
- projects
- secrets
- integrations
- config

## Runtime Plane

Commands that operate on active system runtime resources.

Examples:

- loops
- pods
- documents
- chat sessions

---

# Command Structure

## Top-Level Command Groups

Proposed CLI structure:

```
smithctl auth
smithctl config
smithctl provider
smithctl project
smithctl secret
smithctl integration
smithctl chat
smithctl loop
smithctl pod
smithctl doc
```

---

# CLI Grammar

Commands should follow a consistent grammar:

```
smithctl <resource> <action> [name] [flags]
```

Examples:

```
smithctl provider create openai-work
smithctl project create smith
smithctl loop create
smithctl chat start
```

---

# Authentication Commands

Authentication identifies the operator of the Smith installation.

Example commands:

```
smithctl auth login github
smithctl auth status
smithctl auth logout
```

Authentication should integrate with the GitHub login flow described in the GitHub Authentication PRD.

---

# Provider Commands

Providers represent **Provider Profiles**, reusable configurations for LLM providers.

Example commands:

```
smithctl provider list
smithctl provider get openai-work
smithctl provider create openai-work
smithctl provider update openai-work
smithctl provider delete openai-work
smithctl provider test openai-work
smithctl provider use openai-work
```

Example creation command:

```
smithctl provider create openai-work \
  --type openai \
  --endpoint https://api.openai.com/v1 \
  --model gpt-4.1 \
  --secret openai-key
```

Provider Profiles are referenced by chat sessions, loops, and projects.

---

# Project Commands

Projects represent repository-based runtime contexts used by Smith loops.

Example commands:

```
smithctl project list
smithctl project get smith
smithctl project create smith
smithctl project update smith
smithctl project delete smith
smithctl project use smith
```

Example project creation:

```
smithctl project create smith \
  --repo callmeradical/smith \
  --github-installation default \
  --provider openai-work \
  --replica-image smith-replica:latest \
  --skills-image smith-skills:latest
```

Projects reference Provider Profiles and runtime images.

---

# Secret Commands

Secrets store credentials referenced by providers and integrations.

Example commands:

```
smithctl secret list
smithctl secret get openai-key
smithctl secret set openai-key
smithctl secret delete openai-key
```

Examples:

```
smithctl secret set openai-key --from-env OPENAI_API_KEY
smithctl secret set github-app-key --from-file private-key.pem
```

Secrets may be backed by Kubernetes Secrets.

---

# Integration Commands

Integrations manage external services connected to Smith.

Examples:

```
smithctl integration list
smithctl integration get github
smithctl integration connect github
smithctl integration repos
smithctl integration repos sync
```

Initial integrations include GitHub App connections.

---

# Config Commands

Configuration commands manage installation-level defaults.

Examples:

```
smithctl config get
smithctl config set
smithctl config view
```

Example configuration:

```
smithctl config set default-project smith
smithctl config set default-provider openai-work
smithctl config set default-chat-provider openai-work
```

---

# Chat Commands

Chat commands manage chat sessions and chat configuration.

## Runtime Commands

```
smithctl chat start
smithctl chat attach
smithctl chat send
smithctl chat sessions
smithctl chat stop
```

## Settings Commands

```
smithctl chat settings get
smithctl chat settings set --provider openai-work --thinking balanced
```

---

# Loop Commands

Loops represent autonomous execution workflows.

Example commands:

```
smithctl loop list
smithctl loop create
smithctl loop get
smithctl loop logs
smithctl loop journal
smithctl loop attach
smithctl loop pause
smithctl loop resume
smithctl loop cancel
```

Example loop creation:

```
smithctl loop create --project smith --title "Fix CI failure"
```

Alternative ingress methods:

```
smithctl loop create --project smith --from-github 123
smithctl loop create --project smith --from-prd docs/prd.md
smithctl loop create --project smith --file loop.yaml
```

Advanced overrides remain available for power users.

---

# Context Commands

The CLI may support local context configuration similar to Kubernetes.

Examples:

```
smithctl context view
smithctl context set project smith
smithctl context set provider openai-work
smithctl context set namespace dev
```

Context allows shorter runtime commands.

Example:

```
smithctl loop create --from-github 123
```

---

# Migration Plan

## Phase 1

Introduce new CLI commands without removing existing commands.

Legacy flags remain supported.

## Phase 2

Encourage configuration-first workflows.

Examples:

- create provider profile
- create project
- reference project in loop creation

## Phase 3

Deprecate inline configuration flags for provider and repository credentials.

---

# Success Metrics

- Reduced complexity in loop creation commands
- Increased reuse of provider and project configuration
- Alignment between CLI and UI configuration architecture
- Simplified onboarding workflow

---

# Summary

This PRD introduces a redesigned CLI architecture for `smithctl` that separates configuration management from runtime execution. Durable configuration objects (Providers, Projects, Secrets, Integrations) become first-class CLI resources while runtime commands reference those objects.

This approach aligns the CLI with the Smith UI architecture and reinforces Smith's identity as a runtime control plane.
