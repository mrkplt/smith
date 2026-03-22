<script lang="ts">
  import { onMount } from 'svelte';
  import TopBar from '$lib/components/TopBar.svelte';
  import { appState, pushToast } from '$lib/stores';
  import {
    approveTaskContract,
    createLoopFromTask,
    createTaskContract,
    fetchJSON,
    patchTaskContract,
    type TaskContract
  } from '$lib/api';
  import { isTasksEnabled, isTasksKanbanEnabled } from '$lib/feature-flags';
  import { buildTaskLanes, getTaskTerminalOutcome } from '$lib/tasks/kanban';
  import { goto } from '$app/navigation';

  let tasks = $state<TaskContract[]>([]);
  let loading = $state(false);
  let error = $state('');
  let saving = $state(false);
  let actionTaskID = $state('');
  let editingTaskID = $state('');
  let editObjective = $state('');
  let editValidation = $state('');
  let editSourceDocument = $state('');
  let editProviderProfileID = $state('');

  let projectID = $state('');
  let providerProfileID = $state('codex-default');
  let sourceDocument = $state('docs/task.md');
  let objective = $state('');
  let validationCommands = $state('go test ./...');
  const tasksKanbanEnabled = $derived(isTasksKanbanEnabled());
  const lanes = $derived(buildTaskLanes(tasks, { kanbanEnabled: tasksKanbanEnabled }));

  onMount(async () => {
    if (!isTasksEnabled()) {
      pushToast('Tasks is not enabled in this environment', 'muted');
      await goto('/pods', { replaceState: true });
      return;
    }
    projectID = $appState.projects[0]?.id || 'smith';
    await loadTasks();
  });

  async function loadTasks() {
    loading = true;
    error = '';
    try {
      const response = await fetchJSON('/tasks');
      tasks = Array.isArray(response) ? response : [];
    } catch (err: any) {
      error = err.message || 'Failed to load tasks';
    } finally {
      loading = false;
    }
  }

  function parseMultiline(input: string): string[] {
    return input
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);
  }

  async function createTask() {
    if (!projectID.trim() || !providerProfileID.trim() || !objective.trim()) {
      pushToast('project, provider profile, and objective are required', 'err');
      return;
    }
    const validation = parseMultiline(validationCommands);
    if (validation.length === 0) {
      pushToast('at least one validation command is required', 'err');
      return;
    }
    saving = true;
    try {
      await createTaskContract({
        project_id: projectID.trim(),
        provider_profile_id: providerProfileID.trim(),
        source_document: sourceDocument.trim(),
        objective: objective.trim(),
        validation,
        actor: 'operator'
      });
      objective = '';
      pushToast('task contract created', 'ok');
      await loadTasks();
    } catch (err: any) {
      pushToast(err.message || 'failed to create task', 'err');
    } finally {
      saving = false;
    }
  }

  async function markValidated(task: TaskContract) {
    try {
      await patchTaskContract(task.id, { status: 'validated', actor: 'operator' });
      pushToast('task marked validated', 'ok');
      await loadTasks();
    } catch (err: any) {
      pushToast(err.message || 'failed to update task', 'err');
    }
  }

  async function reopenDraft(task: TaskContract) {
    try {
      await patchTaskContract(task.id, { status: 'draft', actor: 'operator' });
      pushToast('task moved back to draft', 'ok');
      await loadTasks();
    } catch (err: any) {
      pushToast(err.message || 'failed to update task', 'err');
    }
  }

  async function approve(task: TaskContract) {
    try {
      await approveTaskContract(task.id, 'operator');
      pushToast('task approved', 'ok');
      await loadTasks();
    } catch (err: any) {
      pushToast(err.message || 'failed to approve task', 'err');
    }
  }

  async function startLoop(task: TaskContract) {
    actionTaskID = task.id;
    try {
      const idempotencyKey = `task:${task.id}`;
      const result = await createLoopFromTask(task.id, idempotencyKey);
      const loopID = result.loop_id;
      if (!loopID) {
        pushToast('loop created but loop_id missing in response', 'err');
        return;
      }
      pushToast(result.created ? `loop ${loopID} created` : `reusing existing loop ${loopID}`, 'ok');
      await loadTasks();
      await goto(`/pod-view/${encodeURIComponent(loopID)}`);
    } catch (err: any) {
      pushToast(err.message || 'failed to start loop from task', 'err');
    } finally {
      actionTaskID = '';
    }
  }

  function startEdit(task: TaskContract) {
    editingTaskID = task.id;
    editObjective = task.objective || '';
    editValidation = (task.validation || []).join('\n');
    editSourceDocument = task.source_document || '';
    editProviderProfileID = task.provider_profile_id || '';
  }

  function cancelEdit() {
    editingTaskID = '';
    editObjective = '';
    editValidation = '';
    editSourceDocument = '';
    editProviderProfileID = '';
  }

  async function saveEdit(taskID: string) {
    const validation = parseMultiline(editValidation);
    if (!editObjective.trim()) {
      pushToast('objective is required', 'err');
      return;
    }
    if (validation.length === 0) {
      pushToast('at least one validation command is required', 'err');
      return;
    }
    actionTaskID = taskID;
    try {
      await patchTaskContract(taskID, {
        objective: editObjective.trim(),
        validation,
        source_document: editSourceDocument.trim(),
        provider_profile_id: editProviderProfileID.trim(),
        actor: 'operator'
      });
      pushToast('task updated', 'ok');
      cancelEdit();
      await loadTasks();
    } catch (err: any) {
      pushToast(err.message || 'failed to update task', 'err');
    } finally {
      actionTaskID = '';
    }
  }

  function terminalOutcomeLabel(task: TaskContract): string {
    const outcome = getTaskTerminalOutcome(task);
    if (outcome === 'completed') return 'Completed';
    if (outcome === 'blocked') return 'Blocked';
    return 'Terminal';
  }

  function terminalReason(task: TaskContract): string {
    const reason = task.terminal_reason?.trim();
    return reason || '';
  }
</script>

<TopBar title="Tasks" />

<section class="tasks-page px-4 pb-8">
  <div class="panel">
    <h2>Create Task Contract</h2>
    <div class="form-grid">
      <label>
        <span>Project ID</span>
        <input bind:value={projectID} placeholder="smith" />
      </label>
      <label>
        <span>Provider Profile ID</span>
        <input bind:value={providerProfileID} placeholder="codex-default" />
      </label>
      <label class="full">
        <span>Source Document</span>
        <input bind:value={sourceDocument} placeholder="docs/task.md" />
      </label>
      <label class="full">
        <span>Objective</span>
        <textarea rows="2" bind:value={objective} placeholder="Restore green CI on main"></textarea>
      </label>
      <label class="full">
        <span>Validation Commands (one per line)</span>
        <textarea rows="3" bind:value={validationCommands}></textarea>
      </label>
    </div>
    <button class="primary" disabled={saving} onclick={createTask}>
      {saving ? 'Creating…' : 'Create Task'}
    </button>
  </div>

  <div class="panel">
    <div class="panel-head">
      <h2>Task Contracts</h2>
      <button class="ghost" onclick={loadTasks} disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</button>
    </div>
    {#if error}
      <p class="error">{error}</p>
    {/if}
    {#if tasks.length === 0}
      <p class="muted">No task contracts yet.</p>
    {:else}
      <div class="kanban-grid">
        {#each lanes as lane}
          <section class="lane">
            <div class="lane-head">
              <h3>{lane.title}</h3>
              <span class="lane-count">{lane.tasks.length}</span>
            </div>
            {#if lane.tasks.length === 0}
              <p class="muted lane-empty">No tasks</p>
            {:else}
              <div class="task-list">
                {#each lane.tasks as task}
                  <article class="task-item">
                    <div class="task-main">
                      <div class="task-id">{task.id}</div>
                      <div class="task-objective">{task.objective}</div>
                      <div class="task-meta">{task.project_id} · {task.provider_profile_id}</div>
                      {#if editingTaskID === task.id}
                        <div class="edit-grid">
                          <label>
                            <span>Objective</span>
                            <textarea rows="2" bind:value={editObjective}></textarea>
                          </label>
                          <label>
                            <span>Provider Profile</span>
                            <input bind:value={editProviderProfileID} />
                          </label>
                          <label>
                            <span>Source Document</span>
                            <input bind:value={editSourceDocument} />
                          </label>
                          <label>
                            <span>Validation</span>
                            <textarea rows="3" bind:value={editValidation}></textarea>
                          </label>
                        </div>
                      {/if}
                    </div>
                    <div class="task-actions">
                      <span class="status" class:status-blocked={task.status === 'blocked'}>{task.status}</span>
                      {#if lane.id === 'finished'}
                        {@const outcome = getTaskTerminalOutcome(task)}
                        <span class="outcome-pill" class:outcome-blocked={outcome === 'blocked'} class:outcome-completed={outcome === 'completed'}>
                          {terminalOutcomeLabel(task)}
                        </span>
                      {/if}
                      {#if task.status === 'draft'}
                        <button class="ghost" onclick={() => markValidated(task)}>Validate</button>
                      {/if}
                      {#if task.status === 'validated'}
                        <button class="ghost" onclick={() => reopenDraft(task)}>Reopen Draft</button>
                        <button class="primary" onclick={() => approve(task)}>Approve</button>
                      {/if}
                      {#if task.status === 'approved'}
                        <button class="primary" onclick={() => startLoop(task)} disabled={actionTaskID === task.id}>
                          {actionTaskID === task.id ? 'Starting…' : 'Start Loop'}
                        </button>
                      {/if}
                      {#if task.status === 'draft' || task.status === 'validated'}
                        {#if editingTaskID === task.id}
                          <button class="ghost" onclick={cancelEdit}>Cancel Edit</button>
                          <button class="primary" onclick={() => saveEdit(task.id)} disabled={actionTaskID === task.id}>
                            {actionTaskID === task.id ? 'Saving…' : 'Save Edit'}
                          </button>
                        {:else}
                          <button class="ghost" onclick={() => startEdit(task)}>Edit</button>
                        {/if}
                      {/if}
                    </div>
                    {#if lane.id === 'finished'}
                      <details class="terminal-details">
                        <summary>Outcome details</summary>
                        <div class="terminal-row">
                          <span class="terminal-label">Outcome</span>
                          <span>{terminalOutcomeLabel(task)}</span>
                        </div>
                        {#if getTaskTerminalOutcome(task) === 'blocked' && terminalReason(task)}
                          <div class="terminal-row">
                            <span class="terminal-label">Block reason</span>
                            <span>{terminalReason(task)}</span>
                          </div>
                        {/if}
                      </details>
                    {/if}
                  </article>
                {/each}
              </div>
            {/if}
          </section>
        {/each}
      </div>
    {/if}
  </div>
</section>

<style>
  .tasks-page {
    display: grid;
    gap: 1rem;
  }

  .panel {
    border: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(0, 0, 0, 0.75);
    padding: 1rem;
    border-radius: 0.5rem;
  }

  h2 {
    margin: 0 0 0.75rem;
    color: #fff;
    font-size: 0.95rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }

  .form-grid {
    display: grid;
    gap: 0.75rem;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    margin-bottom: 0.75rem;
  }

  .full {
    grid-column: 1 / -1;
  }

  label {
    display: grid;
    gap: 0.35rem;
  }

  label span {
    color: #9ca3af;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  input,
  textarea {
    width: 100%;
    background: rgba(17, 24, 39, 0.7);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #fff;
    border-radius: 0.4rem;
    padding: 0.55rem 0.65rem;
  }

  .task-list {
    display: grid;
    gap: 0.65rem;
  }

  .kanban-grid {
    display: grid;
    gap: 0.75rem;
    grid-template-columns: repeat(5, minmax(12rem, 1fr));
    align-items: start;
    overflow-x: auto;
    padding-bottom: 0.25rem;
  }

  .lane {
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 0.5rem;
    padding: 0.55rem;
    background: rgba(17, 24, 39, 0.25);
    min-height: 8rem;
  }

  .lane-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.55rem;
  }

  h3 {
    margin: 0;
    color: #e5e7eb;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .lane-count {
    border-radius: 999px;
    background: rgba(134, 188, 37, 0.18);
    color: #d9f99d;
    font-size: 0.72rem;
    line-height: 1;
    padding: 0.25rem 0.45rem;
    font-weight: 700;
  }

  .lane-empty {
    margin: 0;
    font-size: 0.8rem;
  }

  .task-item {
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 0.45rem;
    padding: 0.75rem;
    background: rgba(17, 24, 39, 0.35);
  }

  .task-id {
    color: #86bc25;
    font-size: 0.75rem;
    font-weight: 700;
  }

  .task-objective {
    color: #fff;
    margin-top: 0.2rem;
  }

  .task-meta {
    color: #9ca3af;
    font-size: 0.75rem;
    margin-top: 0.25rem;
  }

  .edit-grid {
    display: grid;
    gap: 0.45rem;
    margin-top: 0.6rem;
    grid-template-columns: 1fr 1fr;
  }

  .edit-grid label {
    display: grid;
    gap: 0.25rem;
  }

  .edit-grid span {
    color: #9ca3af;
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .edit-grid textarea,
  .edit-grid input {
    width: 100%;
    background: rgba(17, 24, 39, 0.7);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #fff;
    border-radius: 0.35rem;
    padding: 0.4rem 0.5rem;
    font-size: 0.8rem;
  }

  .task-actions {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .status {
    text-transform: uppercase;
    font-size: 0.68rem;
    letter-spacing: 0.08em;
    color: #d1d5db;
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 999px;
    padding: 0.2rem 0.45rem;
  }

  .status-blocked {
    border-color: rgba(248, 113, 113, 0.65);
    color: #fecaca;
    background: rgba(127, 29, 29, 0.22);
  }

  .outcome-pill {
    text-transform: uppercase;
    font-size: 0.66rem;
    letter-spacing: 0.08em;
    border-radius: 999px;
    padding: 0.2rem 0.45rem;
    border: 1px solid rgba(255, 255, 255, 0.18);
    color: #d1d5db;
    background: rgba(17, 24, 39, 0.4);
  }

  .outcome-completed {
    border-color: rgba(134, 188, 37, 0.55);
    color: #d9f99d;
    background: rgba(77, 124, 15, 0.25);
  }

  .outcome-blocked {
    border-color: rgba(248, 113, 113, 0.65);
    color: #fecaca;
    background: rgba(127, 29, 29, 0.22);
  }

  .terminal-details {
    margin-top: 0.55rem;
    width: 100%;
    font-size: 0.76rem;
    color: #d1d5db;
  }

  .terminal-details summary {
    cursor: pointer;
    color: #9ca3af;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 0.65rem;
  }

  .terminal-row {
    display: grid;
    gap: 0.35rem;
    grid-template-columns: 6.5rem 1fr;
    margin-top: 0.35rem;
  }

  .terminal-label {
    color: #9ca3af;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 0.64rem;
  }

  button {
    border-radius: 0.35rem;
    padding: 0.4rem 0.55rem;
    font-size: 0.75rem;
    cursor: pointer;
  }

  .primary {
    background: #86bc25;
    color: #0b0f15;
    border: 0;
    font-weight: 700;
  }

  .ghost {
    background: transparent;
    color: #d1d5db;
    border: 1px solid rgba(255, 255, 255, 0.2);
  }

  .muted {
    color: #9ca3af;
  }

  .error {
    color: #fca5a5;
    margin-bottom: 0.75rem;
  }

  @media (max-width: 900px) {
    .form-grid {
      grid-template-columns: 1fr;
    }

    .task-item {
      flex-direction: column;
    }

    .task-actions {
      justify-content: flex-start;
    }

    .edit-grid {
      grid-template-columns: 1fr;
    }

    .kanban-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
