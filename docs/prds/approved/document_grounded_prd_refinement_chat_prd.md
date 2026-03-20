---
id: document-grounded-prd-refinement-chat-prd
title: Embedded Guidepost Runtime PRD
status: approved
doc_type: prd
source_doc: docs/planning/approved/document-grounded-prd-refinement-chat.md
target_branch: document-drafting
owner: platform
---

# Embedded Guidepost Runtime (Pi-Powered) — PRD

## 1. Overview

### 1.1 Problem Statement
Current chat-based interactions are insufficient for transforming PRDs into implementation-ready artifacts. The system treats the LLM as a responder rather than an operator, resulting in:
- Loss of structured context
- Inability to safely mutate system state
- Weak integration with build and task workflows

### 1.2 Objective
Introduce an **embedded Guidepost runtime powered by Pi** that operates directly within the system, enabling:
- Document-aware reasoning
- Structured tool execution
- Controlled state mutation
- Autonomous workflow orchestration

### 1.3 Key Shift
From:
> Chat responding to prompts

To:
> Guidepost operating on system state via tools

---

## 2. Architecture

### 2.0 Persistence Decision
The system will use **Postgres as the primary durable store** for Guidepost product state.

Postgres will persist:
- Guidepost sessions
- Messages and event history
- Tool call metadata
- Patch proposals and approvals
- Readiness evaluations
- Task plans
- Build packets

Optional supporting infrastructure:
- **etcd** may be used later for ephemeral coordination concerns such as leases, locks, and active agent ownership, but it is **not** the system of record.


### 2.1 High-Level Design

```
Frontend UI (Guidepost Panel)
        ↓
Guidepost Service (Your System)
        ↓
AgentRuntime (Pi Wrapper)
        ↓
Tool Registry (System APIs)
        ↓
Core Platform (Docs, Tasks, Build, Readiness)
```

---

## 3. Core Concepts

### 3.0 Context Management Model (Critical)

The Guidepost must operate on a structured, multi-layered context model rather than raw prompt text.

#### Context Layers

1. **Stable Context**
   - Workspace
   - Selected document or object
   - Session intent
   - Document version

2. **Activity Context (Focus Context)**
   - Current UI surface (editor, loop viewer, build screen)
   - Active object subtype (PRD, loop, task, build)
   - Focused section or node
   - Selected text or element

3. **Conversational Context**
   - Message history
   - Active clarifications
   - Pending proposals (patches, tasks)

4. **Retrieved Context**
   - On-demand data fetched via tools
   - Document sections
   - Loop definitions
   - Build logs

---

### 3.1 Focus Context

A structured representation of the user’s current UI state.

```json
{
  "surface": "document_editor",
  "entityType": "document",
  "entitySubtype": "prd",
  "entityId": "doc_123",
  "entityVersion": 12,
  "sectionId": "acceptance_criteria",
  "selectionText": "...",
  "uiState": {
    "activePane": "Guidepost",
    "centerTab": "document"
  }
}
```

This context must be continuously updated by the frontend and stored in the Guidepost service.

---

### 3.2 Intent Model

Intent determines Guidepost behavior and is inferred from:

- Explicit user actions (strong signal)
- Current UI surface (moderate signal)
- User message content (weak signal)

#### Supported Intents
- `document_refinement`
- `readiness_review`
- `implementation_planning`
- `loop_debugging`
- `build_diagnosis`
- `general_question`

Example:

```json
{
  "intent": "loop_debugging",
  "confidence": 0.86,
  "signals": [
    "surface=loop_viewer",
    "selected_node=retry_handler",
    "message contains 'retrying forever'"
  ]
}
```

---

### 3.3 Guidepost Context Envelope

The Guidepost runtime receives a synthesized context object:

```json
{
  "session": {},
  "workspace": {},
  "activeObject": {},
  "focus": {},
  "intent": {},
  "recentActivity": [],
  "conversation": {}
}
```

This is generated per agent step and must not include excessive raw UI data.

---

### 3.4 Context Update Events

Frontend must emit structured events:

```json
{
  "type": "ui.context.updated",
  "sessionId": "cp_123",
  "focusContext": {}
}
```

Trigger conditions:
- Active object change
- Section focus change
- Surface/tab change
- User action invocation

---

### 3.5 Context Synthesis

Before each agent step:

1. Load session binding
2. Load latest focus context
3. Summarize recent activity
4. Resolve intent
5. Construct minimal context envelope

---

### 3.6 Context Retrieval Strategy

The agent must not receive full documents by default.

Instead, it should call tools:

- `get_document_section`
- `get_loop_node`
- `get_readiness_report`
- `get_build_logs`

This ensures scalability and avoids prompt bloat.

---

### 3.7 Context Visibility (UX Requirement)

The UI must expose current Guidepost state:

- Bound object (e.g. PRD name + version)
- Focus (e.g. Acceptance Criteria section)
- Mode/intent (e.g. Document Refinement)

This prevents user confusion and improves trust.

---

### 3.1 Document-Bound Session
A Guidepost session explicitly tied to:
- `documentId`
- `documentVersion`
- `workspaceId`

### 3.2 Context Envelope
Constructed by Guidepost Service:

```json
{
  "workspace": {},
  "document": {},
  "readiness": {},
  "intent": "document_refinement"
}
```

### 3.3 Agent Runtime (Pi)
Responsible for:
- Multi-step reasoning loop
- Tool invocation
- Session persistence
- Streaming events

### 3.4 Tool Registry
Defines all system capabilities available to the agent.

---

## 4. Tooling Model (Critical)

### 4.1 Required Tools (MVP)

#### Document Tools
```json
get_document(documentId)
propose_patch(documentId, operations)
apply_patch(patchId)
```

#### Readiness Tools
```json
evaluate_readiness(documentId)
```

#### Planning Tools
```json
generate_tasks(documentId)
```

#### Build Tools
```json
create_build_packet(documentId)
trigger_build(buildPacketId)
```

---

## 5. Agent Loop

### 5.1 Execution Model

```
User Input
   ↓
Agent Think Step
   ↓
Tool Call?
   ↓ YES → Execute Tool → Return Result
   ↓ NO
Final Response + Actions
```

### 5.2 Example Flow

1. Agent calls `get_document`
2. Agent calls `evaluate_readiness`
3. Agent identifies gaps
4. Agent calls `propose_patch`
5. UI presents patch
6. User approves
7. System calls `apply_patch`
8. Agent re-evaluates readiness

---

## 6. Functional Requirements

### 6.1 Session Management
- Bind session to document
- Track version
- Prevent silent context switching

### 6.2 Context Injection
- Inject document content
- Inject readiness diagnostics
- Inject workspace metadata

### 6.3 Tool Execution
- Support structured tool calls
- Validate tool permissions
- Stream tool results

### 6.4 Patch System
- Agent must not directly edit documents
- All changes via patch proposals
- UI approval required

---

## 6.5 Persistence Requirements

### Durable Entities
The system must persist the following entities in Postgres:
- `Guidepost_sessions`
- `Guidepost_events`
- `patch_proposals`
- `patch_approvals`
- `readiness_evaluations`
- `task_plans`
- `build_packets`

### Persistence Principles
- All user and assistant interactions must be recoverable after refresh or reconnect
- All tool invocations must be auditable
- Patch proposals must be versioned and linked to the originating document version
- Readiness evaluations must be historically queryable
- Build packets must be immutable once handed off to execution

## 6.8 Tool Schema & Contracts

### 6.8.1 Design Principles
- All agent actions must be expressed as **typed tool calls**
- Tools must be **idempotent** where possible
- All tool calls must be **auditable** (persisted in `Guidepost_events`)
- Mutations require **explicit user approval** unless flagged safe

### 6.8.2 Tool Interface (TypeScript)

```ts
export type ToolCall<TArgs = any> = {
  name: string;
  args: TArgs;
  callId: string; // correlation id
};

export type ToolResult<TResult = any> = {
  callId: string;
  success: boolean;
  result?: TResult;
  error?: string;
};
```

### 6.8.3 Core Tools (MVP)

#### get_document_section
```json
{
  "name": "get_document_section",
  "args": {
    "documentId": "string",
    "sectionId": "string"
  }
}
```

Returns:
```json
{
  "content": "string",
  "version": 12
}
```

#### evaluate_readiness
```json
{
  "name": "evaluate_readiness",
  "args": {
    "documentId": "string"
  }
}
```

Returns:
```json
{
  "score": 0.64,
  "status": "refine",
  "diagnostics": []
}
```

#### propose_patch
```json
{
  "name": "propose_patch",
  "args": {
    "documentId": "string",
    "operations": []
  }
}
```

Returns:
```json
{
  "patchId": "string",
  "summary": "string"
}
```

#### apply_patch (user-approved)
```json
{
  "name": "apply_patch",
  "args": {
    "patchId": "string"
  }
}
```

#### generate_tasks
```json
{
  "name": "generate_tasks",
  "args": {
    "documentId": "string"
  }
}
```

#### create_build_packet
```json
{
  "name": "create_build_packet",
  "args": {
    "documentId": "string"
  }
}
```

---

## 6.9 Agent Loop (Pi Runtime Integration)

### 6.9.1 Execution Model

The agent runs a **step-based loop** until completion:

```
while (not complete) {
  step = agent.think(context)

  if (step.type === 'tool_call') {
    result = executeTool(step)
    context = updateContext(context, result)
  } else {
    return step.response
  }
}
```

### 6.9.2 Step Types

```ts
type AgentStep =
  | { type: 'message'; content: string }
  | { type: 'tool_call'; tool: ToolCall }
  | { type: 'complete'; output: any };
```

### 6.9.3 Context Injection Per Step

Before each `think()` call, the Guidepost Service must:

1. Load session state
2. Load latest focus context
3. Resolve intent
4. Attach recent tool results
5. Attach minimal conversation history

```ts
function buildContext(sessionId: string): GuidepostContextEnvelope {
  return {
    session: loadSession(sessionId),
    focus: loadFocus(sessionId),
    intent: resolveIntent(sessionId),
    recentActivity: loadRecentEvents(sessionId),
    conversation: loadRecentMessages(sessionId)
  };
}
```

---

## 6.10 Tool Execution Layer

### 6.10.1 Responsibilities
- Validate tool calls
- Enforce permissions
- Execute against system APIs
- Persist results
- Emit websocket events

### 6.10.2 Execution Flow

```ts
async function executeTool(call: ToolCall) {
  persistEvent('tool.call', call);

  const handler = toolRegistry[call.name];
  const result = await handler(call.args);

  persistEvent('tool.result', result);
  return result;
}
```

---

## 6.11 WebSocket Streaming Model

### 6.11.1 Events

```json
assistant.delta
assistant.completed
tool.call
tool.result
patch.proposed
patch.applied
readiness.updated
```

### 6.11.2 Example Sequence

```json
{ "type": "assistant.delta", "text": "Analyzing PRD..." }
{ "type": "tool.call", "name": "evaluate_readiness" }
{ "type": "tool.result", "result": { "score": 0.4 } }
{ "type": "tool.call", "name": "propose_patch" }
{ "type": "patch.proposed", "patchId": "p_123" }
```

---

## 7. API Design

### 7.1 Agent Session

```http
POST /Guidepost/session
```

```json
{
  "documentId": "doc_123",
  "intent": "document_refinement"
}
```

### 7.2 Tool Call Event

```json
{
  "type": "tool.call",
  "name": "propose_patch",
  "args": {}
}
```

### 7.3 Tool Result Event

```json
{
  "type": "tool.result",
  "result": {}
}
```

---

## 8. WebSocket Protocol

### 8.1 Event Types

```json
session.started
context.loaded
assistant.delta
assistant.completed
tool.call
tool.result
document.patch.proposed
readiness.updated
```

---

## 9. UX Design

### 9.1 Guidepost Panel
- Chat stream
- Context badge (Document name + version)
- Session intent indicator

### 9.2 Patch UI
- Diff visualization
- Accept / Reject

### 9.3 System Feedback
- Readiness score updates
- Build readiness indicator

---

## 10. Readiness Model

### Dimensions
- Problem definition
- Scope
- Requirements
- Constraints
- Acceptance criteria
- Edge cases
- Integrations

### Output
- Score (0–1)
- Status (draft, refine, ready, buildable)

---

## 11. Build Packet

```json
{
  "documentId": "doc_123",
  "requirements": [],
  "constraints": [],
  "acceptanceCriteria": [],
  "taskGraph": []
}
```

---

## 12. Implementation Plan

### Phase 1
- Pi runtime integration
- Document-bound session
- Basic tool calling

### Phase 2
- Patch proposal + UI
- Readiness scoring

### Phase 3
- Task generation
- Build packet

### Phase 4
- Autonomous build execution

---

## 12.5 Data Model (Initial)

### `Guidepost_sessions`
- `id`
- `workspace_id`
- `document_id`
- `document_version_at_start`
- `intent`
- `status`
- `created_by`
- `created_at`
- `updated_at`

### `Guidepost_events`
- `id`
- `session_id`
- `event_type`
- `actor_type`
- `actor_id`
- `payload_json`
- `created_at`

### `patch_proposals`
- `id`
- `session_id`
- `document_id`
- `document_version`
- `status`
- `summary`
- `operations_json`
- `created_at`

### `readiness_evaluations`
- `id`
- `document_id`
- `document_version`
- `score`
- `status`
- `diagnostics_json`
- `created_at`

### `build_packets`
- `id`
- `document_id`
- `document_version`
- `packet_json`
- `status`
- `created_at`

## 13. Risks

- Tool misuse by agent
- Poor context construction
- Over-automation without user control

---

## 14. Open Questions

- How to handle large documents?
- Should sessions branch?
- How to version patches?

---

## 15. Future Enhancements

- Multi-document context
- Codebase integration
- Autonomous debugging loops
- Session branching (Pi feature)

---

## 16. Key Principle

**The Guidepost is not chat. It is a system operator powered by an agent runtime (Pi) with controlled access to tools and state.**
