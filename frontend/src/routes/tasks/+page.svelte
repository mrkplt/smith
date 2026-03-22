<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
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
  import { isTasksEnabled } from '$lib/feature-flags';
  import { groupTasksByLane, laneDefinitions } from '$lib/task-lanes';
  import { goto } from '$app/navigation';

  const TASKS_POLL_MS = 5000;

  let tasks = $state<TaskContract[]>([]);
  let loading = $state(false);
  let error = $state('');
  let saving = $state(false);
  let actionTaskID = $state('');
  let editingTaskID = $state('');
  let expandedTaskIDs = $state<string[]>([]);
  let editObjective = $state('');
  let editValidation = $state('');
  let editSourceDocument = $state('');
  let editProviderProfileID = $state('');

  let projectID = $state('');
  let providerProfileID = $state('codex-default');
  let sourceDocument = $state('docs/task.md');
  let objective = $state('');
  let validationCommands = $state('go test ./...');
  const tasksByLane = $derived(groupTasksByLane(tasks));

  let pollTimer: ReturnType<typeof setInterval> | null = null;

  function stopTasksPolling() {
    if (!pollTimer) {
      return;
    }
    clearInterval(pollTimer);
    pollTimer = null;
  }

  function upsertTask(updated: TaskContract) {
    const index = tasks.findIndex((task) => task.id === updated.id);
    if (index === -1) {
      tasks = [updated, ...tasks];
      return;
    }
    const next = [...tasks];
    next[index] = { ...next[index], ...updated };
    tasks = next;
  }

  function markTaskRuntimeRunning(taskID: string) {
    tasks = tasks.map((task) => {
      if (task.id !== taskID) {
        return task;
      }
      return {
        ...task,
        runtime_state: 'running',
        metadata: {
          ...(task.metadata || {}),
          runtime_state: 'running'
        }
      };
    });
  }

  async function loadTasks(options: { silent?: boolean } = {}) {
    const { silent = false } = options;
    if (!silent) {
      loading = true;
      error = '';
    }
    try {
      const response = await fetchJSON('/tasks');
      tasks = Array.isArray(response) ? response : [];
    } catch (err: any) {
      if (!silent) {
        error = err.message || 'Failed to load tasks';
      }
    } finally {
      if (!silent) {
        loading = false;
      }
    }
  }

  function startTasksPolling() {
    stopTasksPolling();
    if (typeof document === 'undefined' || document.hidden) {
      return;
    }
    pollTimer = setInterval(() => {
      void loadTasks({ silent: true });
    }, TASKS_POLL_MS);
  }

  function handleVisibilityChange() {
    if (document.hidden) {
      stopTasksPolling();
      return;
    }
    void loadTasks({ silent: true });
    startTasksPolling();
  }

  function handleWindowFocus() {
    void loadTasks({ silent: true });
  }

  onMount(async () => {
    if (!isTasksEnabled()) {
      pushToast('Tasks is not enabled in this environment', 'muted');
      await goto('/pods', { replaceState: true });
      return;
    }
    projectID = $appState.projects[0]?.id || 'smith';
    await loadTasks();
    startTasksPolling();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    window.addEventListener('focus', handleWindowFocus);
  });

  onDestroy(() => {
    stopTasksPolling();
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener('focus', handleWindowFocus);
    }
  });

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
      const created = await createTaskContract({
        project_id: projectID.trim(),
        provider_profile_id: providerProfileID.trim(),
        source_document: sourceDocument.trim(),
        objective: objective.trim(),
        validation,
        actor: 'operator'
      });
      upsertTask(created);
      objective = '';
      pushToast('task contract created', 'ok');
    } catch (err: any) {
      pushToast(err.message || 'failed to create task', 'err');
    } finally {
      saving = false;
    }
  }

  async function markValidated(task: TaskContract) {
    try {
      const updated = await patchTaskContract(task.id, { status: 'validated', actor: 'operator' });
      upsertTask(updated);
      pushToast('task marked validated', 'ok');
    } catch (err: any) {
      pushToast(err.message || 'failed to update task', 'err');
    }
  }

  async function reopenDraft(task: TaskContract) {
    try {
      const updated = await patchTaskContract(task.id, { status: 'draft', actor: 'operator' });
      upsertTask(updated);
      pushToast('task moved back to draft', 'ok');
    } catch (err: any) {
      pushToast(err.message || 'failed to update task', 'err');
    }
  }

  async function approve(task: TaskContract) {
    try {
      const updated = await approveTaskContract(task.id, 'operator');
      upsertTask(updated);
      pushToast('task approved', 'ok');
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
      markTaskRuntimeRunning(task.id);
      pushToast(result.created ? `loop ${loopID} created` : `reusing existing loop ${loopID}`, 'ok');
      void loadTasks({ silent: true });
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

  function isExpanded(taskID: string): boolean {
    return expandedTaskIDs.includes(taskID);
  }

  function toggleExpanded(taskID: string) {
    if (isExpanded(taskID)) {
      expandedTaskIDs = expandedTaskIDs.filter((id) => id !== taskID);
      return;
    }
    expandedTaskIDs = [...expandedTaskIDs, taskID];
  }

  function readTaskMetadata(task: TaskContract, keys: string[]): string {
    if (!task.metadata) {
      return '';
    }
    for (const key of keys) {
      const value = task.metadata[key];
      if (typeof value === 'string' && value.trim() !== '') {
        return value.trim();
      }
    }
    return '';
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
      const updated = await patchTaskContract(taskID, {
        objective: editObjective.trim(),
        validation,
        source_document: editSourceDocument.trim(),
        provider_profile_id: editProviderProfileID.trim(),
        actor: 'operator'
      });
      upsertTask(updated);
      pushToast('task updated', 'ok');
      cancelEdit();
    } catch (err: any) {
      pushToast(err.message || 'failed to update task', 'err');
    } finally {
      actionTaskID = '';
    }
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
      <button class="ghost" onclick={() => loadTasks()} disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</button>
    </div>
    {#if error}
      <p class="error">{error}</p>
    {/if}
    <div class="kanban-board">
      {#each laneDefinitions as lane}
        <section class="kanban-lane">
          <div class="lane-head">
            <h3>{lane.label}</h3>
            <span class="lane-count">{tasksByLane[lane.id].length}</span>
          </div>
          <div class="lane-body">
            {#if tasksByLane[lane.id].length === 0}
              <p class="muted">No tasks</p>
            {:else}
              <div class="task-list">
                {#each tasksByLane[lane.id] as task}
                  <article class="task-item">
                    <div class="task-main">
                      <div class="task-id">{task.id}</div>
                      <div class="task-objective">{task.objective}</div>
                      <div class="task-meta">{task.project_id} · {task.provider_profile_id}</div>
                      {#if lane.id === 'scheduled'}
                        <div class="task-planning">
                          {#if readTaskMetadata(task, ['priority'])}
                            <span>Priority: {readTaskMetadata(task, ['priority'])}</span>
                          {/if}
                          {#if readTaskMetadata(task, ['planned_launch_date', 'planned_launch_at', 'planned_launch'])}
                            <span>
                              Planned Launch:
                              {readTaskMetadata(task, ['planned_launch_date', 'planned_launch_at', 'planned_launch'])}
                            </span>
                          {/if}
                        </div>
                      {/if}
                      {#if isExpanded(task.id)}
                        <div class="task-details">
                          <div><strong>Status:</strong> {task.status}</div>
                          {#if task.runtime_state}
                            <div><strong>Runtime State:</strong> {task.runtime_state}</div>
                          {/if}
                          {#if task.source_document}
                            <div><strong>Source:</strong> {task.source_document}</div>
                          {/if}
                          {#if (task.validation || []).length > 0}
                            <div><strong>Validation:</strong> {(task.validation || []).join(' · ')}</div>
                          {/if}
                        </div>
                      {/if}
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
                      <span class="status">{task.status}</span>
                      <button class="ghost" onclick={() => toggleExpanded(task.id)}>
                        {isExpanded(task.id) ? 'Hide Details' : 'View Details'}
                      </button>
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
                  </article>
                {/each}
              </div>
            {/if}
          </div>
        </section>
      {/each}
    </div>
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

  .kanban-board {
    display: grid;
    grid-template-columns: repeat(7, minmax(280px, 1fr));
    gap: 0.75rem;
    overflow-x: auto;
    padding-bottom: 0.25rem;
  }

  .kanban-lane {
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 0.5rem;
    background: rgba(17, 24, 39, 0.25);
    display: grid;
    grid-template-rows: auto 1fr;
    min-height: 12rem;
  }

  .lane-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.65rem 0.75rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }

  .lane-head h3 {
    margin: 0;
    font-size: 0.74rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #d1d5db;
  }

  .lane-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1.5rem;
    border-radius: 999px;
    font-size: 0.72rem;
    font-weight: 700;
    padding: 0.1rem 0.4rem;
    background: rgba(134, 188, 37, 0.16);
    color: #d9f99d;
  }

  .lane-body {
    padding: 0.65rem;
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

  .task-planning {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
    margin-top: 0.35rem;
  }

  .task-planning span {
    color: #d9f99d;
    background: rgba(134, 188, 37, 0.12);
    border: 1px solid rgba(134, 188, 37, 0.25);
    border-radius: 999px;
    padding: 0.15rem 0.45rem;
    font-size: 0.7rem;
    line-height: 1.2;
  }

  .task-details {
    margin-top: 0.45rem;
    display: grid;
    gap: 0.25rem;
    color: #d1d5db;
    font-size: 0.76rem;
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

    .kanban-board {
      grid-template-columns: repeat(7, minmax(240px, 1fr));
    }
  }
</style>
