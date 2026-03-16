---
id: smith-github-auth-prd
title: Smith GitHub Authentication and Repository Authorization PRD
status: approved
doc_type: prd
source_doc: docs/planning/approved/smith-github-authentication.md
target_branch: chat-wip
owner: lars
---

# Product Requirements Document (PRD)

# Smith GitHub Authentication and Repository Authorization

## Overview

Smith currently onboards projects by pairing a repository with a Personal Access Token (PAT). While this approach is functional for early development, it introduces security, operational, and scalability concerns when used in larger environments or enterprise contexts.

This PRD proposes replacing the PAT-first onboarding model with a GitHub App–based integration and GitHub authentication for Smith installations.

The goal is to provide a secure, modern, and developer-friendly authentication model that aligns with GitHub best practices while maintaining Smith's architecture as a single-developer installation runtime.

---

# Goals

1. Replace repository + PAT onboarding with GitHub App installation.
2. Provide GitHub-based authentication for Smith operators.
3. Support repository-scoped authorization rather than account-scoped tokens.
4. Eliminate long-lived GitHub credentials stored in Smith.
5. Maintain Smith's single-developer installation model.
6. Provide an enterprise-compatible security model.

---

# Non-Goals

- Building a full multi-user authentication system inside Smith
- Implementing internal RBAC or organization management
- Managing enterprise identity providers directly
- Replacing Kubernetes-level security or tenancy controls

Smith remains a **single-operator installation runtime**.

---

# Current State

Project onboarding currently works as follows:

1. Developer installs Smith
2. Developer registers a project by providing:

   - Repository URL
   - Personal Access Token (PAT)

3. Smith stores the PAT as a Kubernetes secret
4. Smith uses the PAT for:

   - cloning repositories
   - interacting with GitHub APIs
   - creating pull requests
   - reading issues


### Limitations

- PATs are long-lived
- PATs often have broad access scopes
- Revocation requires manual rotation
- PATs are tied to a specific user account
- Secret sprawl inside Kubernetes
- Weak enterprise story

---

# Proposed Architecture

Smith will integrate with GitHub using the following model:

1. Developer authenticates to Smith using GitHub OAuth
2. Developer installs the Smith GitHub App
3. Developer selects repositories to authorize
4. Smith generates short-lived installation tokens
5. Smith uses installation tokens for all GitHub interactions


## Key Principle

Smith does not manage developer identity.

GitHub provides:

- Developer authentication
- Repository authorization

Smith manages:

- projects
- loops
- runtime configuration
- providers
- orchestration

---

# System Components

## GitHub OAuth Login

Used only for identifying the operator of the Smith installation.

Responsibilities:

- authenticate developer
- retrieve GitHub identity
- establish Smith session

Data stored:

- GitHub user id
- GitHub username

---

## Smith GitHub App

The Smith GitHub App provides repository-level authorization.

Responsibilities:

- access repository contents
- read/write pull requests
- read issues
- post comments

Permissions should be minimal and configurable.

Typical permissions:

- contents: read/write
- pull_requests: write
- issues: read

---

## GitHub App Installation

Users install the Smith GitHub App into:

- personal repositories
- organization repositories

Users explicitly choose which repositories Smith can access.

Smith records the installation ID.

---

## Installation Tokens

Smith will generate GitHub App installation tokens dynamically.

Characteristics:

- scoped to a GitHub App installation
- scoped to selected repositories
- short-lived (~1 hour)

Smith never stores long-lived GitHub tokens.

---

# Updated Project Onboarding Flow

## Current Flow

Add Project

```
repo_url
PAT
```

---

## New Flow

Add Project

1. Sign into Smith via GitHub
2. Install Smith GitHub App
3. Select repositories
4. Choose repository inside Smith


Smith stores:

- installation_id
- repository_name
- repository_owner

Smith retrieves tokens dynamically when performing GitHub operations.

---

# User Stories

## Story 1

As a developer

I want to sign into Smith using my GitHub account

So that I do not need to manage separate credentials.

### Acceptance Criteria

- GitHub login button available in Smith UI
- OAuth flow completes successfully
- Smith stores GitHub identity metadata

---

## Story 2

As a developer

I want to install the Smith GitHub App on specific repositories

So that Smith can interact with my code.

### Acceptance Criteria

- User redirected to GitHub App installation page
- User can choose repositories
- Installation ID returned to Smith

---

## Story 3

As a developer

I want to select a repository from authorized repos

So that Smith can manage loops against that project.

### Acceptance Criteria

- Smith lists repos available to installation
- User selects repo
- Repo stored as Smith project

---

## Story 4

As a developer

I want Smith to create pull requests automatically

So that loop results can be committed safely.

### Acceptance Criteria

- Smith generates installation token
- Smith pushes branch
- Smith creates PR

---

# Security Model

## Token Management

Smith only uses:

- GitHub App private key
- Installation tokens

Smith does not store:

- personal access tokens
- GitHub passwords

---

## Secret Storage

Secrets stored in Kubernetes:

- GitHub App private key
- installation metadata

Installation tokens are generated on demand.

---

# Backwards Compatibility

During transition Smith should support:

1. GitHub App onboarding (preferred)
2. Fine-grained PAT onboarding (fallback)

Classic PAT usage should be deprecated.

---

# Risks

## GitHub App Permission Complexity

Developers unfamiliar with GitHub Apps may find the installation process confusing.

Mitigation:

- clear onboarding wizard
- documentation

---

## Enterprise GitHub Restrictions

Some organizations restrict GitHub App installations.

Mitigation:

- fallback PAT option

---

# Open Questions

1. Should Smith support GitHub Enterprise Server?
2. Should multiple installations be supported per Smith instance?
3. Should Smith automatically discover repositories?
4. Should GitHub webhooks trigger loop execution?

---

# Success Metrics

- Reduction in PAT usage
- Faster onboarding time
- Reduced secret management overhead
- Increased enterprise adoption

---

# Summary

This proposal replaces PAT-based repository authentication with a GitHub App integration while using GitHub OAuth for developer authentication.

The new model improves security, simplifies onboarding, and aligns Smith with modern GitHub integration patterns while preserving Smith's single-developer installation architecture.
