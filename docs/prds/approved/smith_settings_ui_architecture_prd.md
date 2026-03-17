---
id: smith-settings-ui-architecture-prd
title: Smith Settings Providers and Projects UI Architecture PRD
status: approved
doc_type: prd
source_doc: docs/planning/approved/smith-settings-ui-architecture.md
target_branch: chat-wip
owner: lars
---

# Product Requirements Document (PRD)

# Smith Settings, Providers, and Projects UI Architecture

## Overview

Smith is evolving into a runtime control plane for autonomous development loops, chat agents, and repository automation. As the system grows, the current UI navigation and configuration structure will not scale well unless configuration objects are organized under a unified settings architecture.

Currently the top navigation exposes:

Pods | Documents | Projects | Providers | Settings

However, Projects and Providers are not runtime surfaces. They are configuration primitives that define how Smith interacts with repositories, providers, and runtimes.

This PRD proposes reorganizing the UI so that:

• Runtime surfaces remain in the primary navigation
• Configuration objects move into the Settings area
• Provider configuration becomes reusable "Provider Profiles"
• Projects reference Provider Profiles
• Configuration is managed via drawer-based editors

The goal is to create a scalable UI architecture that supports future features without constant navigation refactoring.

---

# Goals

1. Reorganize the Smith UI navigation hierarchy.
2. Move Projects and Providers under Settings.
3. Introduce Provider Profiles as the canonical provider abstraction.
4. Preserve the existing card + drawer configuration interaction model.
5. Support future integrations (GitHub App, secrets, runtime configuration).
6. Maintain Smith's identity as a runtime control plane rather than a traditional CRUD SaaS UI.

---

# Non-Goals

- Redesigning the overall visual theme
- Replacing the card + drawer interaction pattern
- Introducing a full multi-user permission system

The current visual language (dark terminal aesthetic with neon green accent) will remain unchanged.

---

# UI Philosophy

Smith should feel like a:

Kubernetes Dashboard + AI Control Plane

Not a traditional SaaS configuration dashboard.

The UI should clearly separate:

Runtime surfaces (things happening in the system)

Configuration surfaces (things defining system behavior)

---

# Navigation Architecture

## Current Navigation

Pods | Documents | Projects | Providers | Settings

## Proposed Navigation

Pods | Documents | Settings | Chat

Runtime surfaces remain in the top navigation.

Configuration objects move under Settings.

---

# Settings Navigation

Inside the Settings page a sidebar menu will be introduced.

Example:

Settings

General
Providers
Projects
Chat
Integrations
Secrets

This sidebar enables the Settings area to scale without increasing top navigation complexity.

---

# Settings Sections

## General

Installation-level configuration.

Example fields:

Smith Installation ID
Cluster Context
Runtime Namespace
Operator Identity
Smith Version
Upgrade Channel

Future capabilities:

GitHub login status
Runtime health
Cluster connection status

---

## Providers

This section manages **Provider Profiles**.

Provider Profiles represent reusable configurations for LLM providers.

Examples:

OpenAI – Personal
OpenAI – Work
Anthropic – Default
OpenAI Gateway – Local

Provider profiles are referenced by:

Chat agents
Loops
Projects
Automation workers

### Provider Card

Each provider profile is represented as a card.

Card fields:

Profile Name
Provider Type
Default Model
Capabilities
Connection Status

Example:

OpenAI – Personal
Provider: OpenAI
Model: gpt-4.1
Capabilities: Chat • Tools

---

## Provider Configuration Drawer

Clicking "Edit" or "Configure" opens a right-side drawer.

Example layout:

Configure Provider Profile

Profile Name
Provider Type
Endpoint
Default Model
Capabilities
Secret Reference

Optional fields:

API version
Custom headers
Provider-specific metadata

Secrets are referenced rather than stored directly in the UI.

---

# Projects

Projects represent repository-based runtime contexts used by Smith loops and automation.

Projects are configuration primitives and therefore belong inside Settings.

---

## Project Card

Each project appears as a card.

Fields:

Project Name
Repository
Provider Profile
Runtime

Example:

SMITH
Repository: github.com/callmeradical/smith
Provider Profile: OpenAI – Work
Runtime: Default

---

# Project Configuration Drawer

Projects are configured using the existing right-side drawer pattern.

Example fields:

Project Name
Repository URL
GitHub Installation
Provider Profile

Runtime Section:

Replica Image
Skills Image

The runtime configuration allows projects to control execution environments.

---

# Authentication Model

Projects will migrate away from PAT-based authentication.

Current fields:

Git User
Secret Token

Future model:

GitHub App Installation
Repository Selection
Provider Profile

This aligns with the GitHub App integration model defined in the GitHub Authentication PRD.

---

# Chat Settings

The existing Chat Defaults page becomes **Chat Settings**.

This section defines default operator chat behavior.

Fields:

Provider Profile
Default Model
Thinking Level
Session API Key Override
Full Screen Chat Default

The provider selection references Provider Profiles instead of raw providers.

---

# Secrets

Secrets are managed centrally in this section.

Examples:

openai-key
anthropic-key
github-app-key

Provider Profiles reference secrets instead of storing credentials.

Secrets may be backed by Kubernetes Secrets.

---

# Integrations

External system connections.

Examples:

GitHub
Slack
Jira
Linear

Initial focus:

GitHub App installation
Repository access
Webhook configuration

---

# UI Interaction Patterns

## Entity Cards

Entities such as Providers and Projects are displayed as cards.

Cards show summary information and configuration state.

Example actions:

Edit
Configure
View Status

---

## Drawer Editors

Configuration editing occurs in right-side drawers.

This pattern is already implemented in Smith and will remain the primary interaction model.

Benefits:

Non-disruptive editing
Consistent interaction pattern
Clear separation between browsing and editing

---

# Runtime Status Indicators (Future Enhancement)

Smith may introduce a runtime status bar showing:

Cluster Connection
Active Pods
Provider Connectivity
GitHub Integration Status

This reinforces Smith's identity as a control plane.

---

# Success Metrics

• Reduced top navigation complexity
• Reusable provider configuration
• Elimination of duplicated provider setup
• Simplified onboarding of projects and providers
• Support for future integrations without navigation redesign

---

# Summary

This PRD introduces a scalable configuration architecture for Smith by:

• Moving Projects and Providers under Settings
• Introducing Provider Profiles
• Maintaining the card + drawer interaction model
• Separating runtime surfaces from configuration surfaces

The resulting UI architecture supports future growth while preserving Smith's identity as a runtime control plane.
