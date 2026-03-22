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
  function toSortTimestamp(task: TaskContract): number {
    const timestamp = task.validated_at || task.updated_at || task.created_at;
    if (!timestamp) {
      return Number.MAX_SAFE_INTEGER;
    }
    const parsed = Date.parse(timestamp);
    return Number.isNaN(parsed) ? Number.MAX_SAFE_INTEGER : parsed;
  }

  function compareAwaitingApprovalTasks(a: TaskContract, b: TaskContract): number {
    const priorityDelta = (b.review_priority ?? 0) - (a.review_priority ?? 0);
    if (priorityDelta !== 0) {
      return priorityDelta;
    }
    const timestampDelta = toSortTimestamp(a) - toSortTimestamp(b);
    if (timestampDelta !== 0) {
      return timestampDelta;
    }
    return a.id.localeCompare(b.id);
  }

  let awaitingApprovalTasks = $derived(
    tasks
      .filter((task) => task.status === 'validated')
      .slice()
      .sort(compareAwaitingApprovalTasks)
  );
  let draftTasks = $derived(tasks.filter((task) => task.status === 'draft'));
  let approvedTasks = $derived(tasks.filter((task) => task.status === 'approved'));
  let runningTasks = $derived(tasks.filter((task) => task.status === 'running'));
  let completedTasks = $derived(tasks.filter((task) => task.status === 'completed'));
  let blockedTasks = $derived(tasks.filter((task) => task.status === 'blocked'));

  let projectID = $state('');
  let providerProfileID = $state('codex-default');
  let sourceDocument = $state('docs/task.md');
  let objective = $state('');
  let validationCommands = $state('go test ./...');

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
</script>

<TopBar title="Tasks" />

<section class="tasks-page px-4 pb-8">
  {#if isTasksKanbanEnabled()}
    <div class="panel">
      <h2>Kanban Board</h2>
      <div class="kanban-lanes" data-testid="kanban-lanes">
        <section class="kanban-lane" aria-label="Awaiting Approval lane" data-testid="awaiting-approval-lane">
          <h3>
            Awaiting Approval
            <span class="lane-count" aria-hidden="true">({awaitingApprovalTasks.length})</span>
          </h3>
          <div class="kanban-cards">
            {#each awaitingApprovalTasks as task (task.id)}
              <article class="kanban-card" data-testid="awaiting-approval-card">
                <div class="task-id">{task.id}</div>
                <div class="task-objective">{task.objective}</div>
              </article>
            {/each}
            {#if awaitingApprovalTasks.length === 0}
              <p class="lane-empty" data-testid="awaiting-approval-empty">No tasks awaiting approval.</p>
            {/if}
          </div>
        </section>
        <section class="kanban-lane" aria-label="Draft lane" data-testid="draft-lane">
          <h3>
            Draft
            <span class="lane-count" aria-hidden="true">({draftTasks.length})</span>
          </h3>
          <div class="kanban-cards">
            {#each draftTasks as task (task.id)}
              <article class="kanban-card" data-testid="draft-card">
                <div class="task-id">{task.id}</div>
                <div class="task-objective">{task.objective}</div>
              </article>
            {/each}
          </div>
        </section>
        <section class="kanban-lane" aria-label="Approved lane" data-testid="approved-lane">
          <h3>
            Approved
            <span class="lane-count" aria-hidden="true">({approvedTasks.length})</span>
          </h3>
          <div class="kanban-cards">
            {#each approvedTasks as task (task.id)}
              <article class="kanban-card" data-testid="approved-card">
                <div class="task-id">{task.id}</div>
                <div class="task-objective">{task.objective}</div>
              </article>
            {/each}
          </div>
        </section>
        <section class="kanban-lane" aria-label="Running lane" data-testid="running-lane">
          <h3>
            Running
            <span class="lane-count" aria-hidden="true">({runningTasks.length})</span>
          </h3>
          <div class="kanban-cards">
            {#each runningTasks as task (task.id)}
              <article class="kanban-card" data-testid="running-card">
                <div class="task-id">{task.id}</div>
                <div class="task-objective">{task.objective}</div>
              </article>
            {/each}
          </div>
        </section>
        <section class="kanban-lane" aria-label="Completed lane" data-testid="completed-lane">
          <h3>
            Completed
            <span class="lane-count" aria-hidden="true">({completedTasks.length})</span>
          </h3>
          <div class="kanban-cards">
            {#each completedTasks as task (task.id)}
              <article class="kanban-card" data-testid="completed-card">
                <div class="task-id">{task.id}</div>
                <div class="task-objective">{task.objective}</div>
              </article>
            {/each}
          </div>
        </section>
        <section class="kanban-lane" aria-label="Blocked lane" data-testid="blocked-lane">
          <h3>
            Blocked
            <span class="lane-count" aria-hidden="true">({blockedTasks.length})</span>
          </h3>
          <div class="kanban-cards">
            {#each blockedTasks as task (task.id)}
              <article class="kanban-card" data-testid="blocked-card">
                <div class="task-id">{task.id}</div>
                <div class="task-objective">{task.objective}</div>
              </article>
            {/each}
          </div>
        </section>
      </div>
    </div>
  {/if}

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
      <div class="task-list">
        {#each tasks as task}
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

  .kanban-lanes {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 0.75rem;
  }

  .kanban-lane {
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 0.45rem;
    background: rgba(17, 24, 39, 0.35);
    min-height: 5rem;
    padding: 0.75rem;
  }

  .kanban-lane h3 {
    margin: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: #d1d5db;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .lane-count {
    color: #9ca3af;
    font-size: 0.72rem;
    letter-spacing: 0.03em;
  }

  .kanban-cards {
    display: grid;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .kanban-card {
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 0.45rem;
    background: rgba(17, 24, 39, 0.35);
    padding: 0.6rem;
  }

  .lane-empty {
    margin: 0;
    color: #9ca3af;
    font-size: 0.8rem;
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
  }
</style>
