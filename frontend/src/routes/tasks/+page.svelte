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
  import { isTasksEnabled } from '$lib/feature-flags';
  import { resolveTaskLane, type TaskLane } from '$lib/tasks/lane-mapping';
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
  const TASKS_POLL_INTERVAL_MS = 10_000;

  let pollIntervalHandle: ReturnType<typeof setInterval> | undefined;

  const KANBAN_LANES: { id: TaskLane; label: string; emptyMessage: string }[] = [
    { id: 'in_focus', label: 'In Focus', emptyMessage: 'No tasks currently in active execution.' },
    { id: 'backlog', label: 'Backlog', emptyMessage: 'No tasks in backlog.' },
    { id: 'blocked', label: 'Blocked', emptyMessage: 'No blocked tasks.' },
    { id: 'done', label: 'Done', emptyMessage: 'No completed tasks yet.' }
  ];

  onMount(() => {
    const initializeTasks = async () => {
      if (!isTasksEnabled()) {
        pushToast('Tasks is not enabled in this environment', 'muted');
        await goto('/pods', { replaceState: true });
        return;
      }

      projectID = $appState.projects[0]?.id || 'smith';
      await loadTasks();
      startTaskPolling();
    };

    void initializeTasks();

    const handleVisibilityChange = () => {
      if (document.hidden) {
        stopTaskPolling();
        return;
      }

      void loadTasks();
      startTaskPolling();
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      stopTaskPolling();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
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

  function startTaskPolling() {
    if (pollIntervalHandle || document.hidden) {
      return;
    }

    pollIntervalHandle = setInterval(() => {
      if (loading || saving) {
        return;
      }
      void loadTasks();
    }, TASKS_POLL_INTERVAL_MS);
  }

  function stopTaskPolling() {
    if (!pollIntervalHandle) {
      return;
    }
    clearInterval(pollIntervalHandle);
    pollIntervalHandle = undefined;
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

  function tasksForLane(lane: TaskLane): TaskContract[] {
    return tasks.filter((task) => resolveTaskLane(task.status) === lane);
  }

  function currentStepLabel(task: TaskContract): string {
    const currentStep = task.metadata?.current_step?.trim();
    return currentStep ? `Step: ${currentStep}` : 'Step unavailable';
  }

  function recencyLabel(task: TaskContract): string {
    if (!task.updated_at) {
      return 'Update time unavailable';
    }

    const updatedAtMs = Date.parse(task.updated_at);
    if (Number.isNaN(updatedAtMs)) {
      return 'Update time unavailable';
    }

    const elapsedSeconds = Math.max(0, Math.floor((Date.now() - updatedAtMs) / 1000));
    if (elapsedSeconds < 60) {
      return 'Updated just now';
    }

    if (elapsedSeconds < 60 * 60) {
      return `Updated ${Math.floor(elapsedSeconds / 60)}m ago`;
    }

    if (elapsedSeconds < 60 * 60 * 24) {
      return `Updated ${Math.floor(elapsedSeconds / (60 * 60))}h ago`;
    }

    return `Updated ${Math.floor(elapsedSeconds / (60 * 60 * 24))}d ago`;
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
    {/if}
    <div class="kanban-board" data-testid="tasks-kanban-board">
      {#each KANBAN_LANES as lane}
        {@const laneTasks = tasksForLane(lane.id)}
        <section class="lane" data-testid={`lane-${lane.id}`}>
          <header class="lane-head">
            <h3>{lane.label}</h3>
            <span class="lane-count">{laneTasks.length}</span>
          </header>
          {#if laneTasks.length === 0}
            <p class="lane-empty" data-testid={`lane-empty-${lane.id}`}>{lane.emptyMessage}</p>
          {:else}
            <div class="lane-list" data-testid={`lane-cards-${lane.id}`}>
              {#each laneTasks as task}
                <article class="task-item" data-testid={`task-card-${lane.id}`}>
                  <div class="task-main">
                    <div class="task-id">{task.id}</div>
                    <div class="task-objective">{task.objective}</div>
                    <div class="task-meta">{task.project_id} · {task.provider_profile_id}</div>
                    {#if lane.id === 'in_focus'}
                      <div class="task-meta task-in-focus-meta" data-testid="task-in-focus-current-step">
                        {currentStepLabel(task)}
                      </div>
                      <div class="task-meta task-in-focus-meta" data-testid="task-in-focus-recency">
                        {recencyLabel(task)}
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

  .kanban-board {
    display: grid;
    gap: 0.75rem;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    align-items: start;
  }

  .lane {
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 0.5rem;
    background: rgba(17, 24, 39, 0.25);
    padding: 0.65rem;
    min-height: 8rem;
  }

  .lane-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.6rem;
  }

  h3 {
    margin: 0;
    color: #e5e7eb;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .lane-count {
    color: #d1d5db;
    font-size: 0.75rem;
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 999px;
    padding: 0.1rem 0.4rem;
  }

  .lane-empty {
    color: #9ca3af;
    font-size: 0.8rem;
    margin: 0;
  }

  .lane-list {
    display: grid;
    gap: 0.65rem;
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

  .task-in-focus-meta {
    color: #cbd5e1;
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

    .kanban-board {
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
  }
</style>
