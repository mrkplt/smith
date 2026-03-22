<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import TopBar from '$lib/components/TopBar.svelte';
  import { fetchJSON, type TaskContract } from '$lib/api';
  import { isTasksKanbanVisible } from '$lib/feature-flags';
  import { resolveTaskLane, type TaskLane } from '$lib/tasks/lane-mapping';
  import { pushToast } from '$lib/stores';

  const REFRESH_INTERVAL_MS = 10_000;

  type KanbanLaneMeta = {
    id: TaskLane;
    title: string;
    empty: string;
  };

  type KanbanCard = {
    id: string;
    objective: string;
    projectID: string;
    providerProfileID: string;
    status: string;
    lane: TaskLane;
    updatedAt: string;
    createdAt: string;
    step: string;
    runtimeState: string;
    task: TaskContract;
  };

  const KANBAN_LANES: readonly KanbanLaneMeta[] = [
    { id: 'in_focus', title: 'In Focus', empty: 'No tasks currently in active execution.' },
    { id: 'backlog', title: 'Scheduled', empty: 'No scheduled tasks.' },
    { id: 'blocked', title: 'Blocked', empty: 'No blocked tasks.' },
    { id: 'done', title: 'Finished', empty: 'No finished tasks yet.' }
  ] as const;

  let tasks = $state<TaskContract[]>([]);
  let error = $state('');
  let accessValidated = $state(false);
  let selectedCardID = $state('');
  let laneFilter = $state<'all' | TaskLane>('all');
  let searchQuery = $state('');

  let pollTimer: ReturnType<typeof setInterval> | null = null;
  const cards = $derived.by<KanbanCard[]>(() => {
    return tasks.map(toTaskCard);
  });

  const visibleCards = $derived.by<KanbanCard[]>(() => {
    const query = searchQuery.trim().toLowerCase();
    return cards.filter((card) => {
      const laneMatches = laneFilter === 'all' || card.lane === laneFilter;
      if (!laneMatches) {
        return false;
      }
      if (!query) {
        return true;
      }
      const searchable = [
        card.id,
        card.objective,
        card.projectID,
        card.providerProfileID,
        card.status,
        documentRefForCard(card),
        loopIDForCard(card)
      ]
        .join(' ')
        .toLowerCase();
      return searchable.includes(query);
    });
  });

  const lanes = $derived.by(() => {
    const grouped: Record<TaskLane, KanbanCard[]> = {
      in_focus: [],
      backlog: [],
      blocked: [],
      done: []
    };

    for (const card of visibleCards) {
      grouped[card.lane].push(card);
    }

    for (const laneTasks of Object.values(grouped)) {
      laneTasks.sort((a, b) => toEpoch(b.updatedAt) - toEpoch(a.updatedAt));
    }

    return KANBAN_LANES.map((lane) => ({
      ...lane,
      tasks: grouped[lane.id]
    }));
  });

  const selectedCard = $derived.by<KanbanCard | null>(() => {
    if (!selectedCardID) {
      return null;
    }
    return visibleCards.find((card) => card.id === selectedCardID) ?? null;
  });

  $effect(() => {
    if (selectedCardID && !visibleCards.some((card) => card.id === selectedCardID)) {
      selectedCardID = '';
    }
  });

  onMount(async () => {
    if (!isTasksKanbanVisible()) {
      console.info('[feature-flag] feature=tasks-kanban action=route-block redirect=/pods reason=disabled');
      pushToast('Tasks Kanban is not enabled in this environment', 'muted');
      await goto('/pods', { replaceState: true });
      return;
    }

    accessValidated = true;
    await loadTasks('initial');
    pollTimer = setInterval(() => {
      void loadTasks('poll');
    }, REFRESH_INTERVAL_MS);
  });

  onDestroy(() => {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  });

  async function loadTasks(reason: 'initial' | 'poll'): Promise<void> {
    try {
      const payload = await fetchJSON('/v1/tasks');
      tasks = Array.isArray(payload) ? payload : [];
      error = '';
    } catch (err: any) {
      error = err?.message || 'Failed to load tasks';
      if (reason === 'initial') {
        pushToast(error, 'err');
      }
    }
  }

  function taskTimestamp(task: TaskContract): string {
    return task.updated_at || task.created_at || '';
  }

  function toTaskCard(task: TaskContract): KanbanCard {
    const step = task.metadata?.current_step?.trim();
    const topLevelState = String((task as any).runtime_state || '').trim();
    const metadataState = (task.metadata?.runtime_state || '').trim();
    const runtimeState = topLevelState || metadataState;

    return {
      id: task.id,
      objective: task.objective || task.id,
      projectID: task.project_id || 'default',
      providerProfileID: task.provider_profile_id || 'unknown',
      status: task.status,
      lane: resolveTaskLane(task.status),
      updatedAt: taskTimestamp(task),
      createdAt: task.created_at || '',
      step: step ? `Step: ${step}` : 'Step unavailable',
      runtimeState: runtimeState ? `State: ${runtimeState}` : '',
      task
    };
  }

  function toEpoch(value: string): number {
    if (!value) {
      return 0;
    }
    const parsed = Date.parse(value);
    return Number.isNaN(parsed) ? 0 : parsed;
  }

  function formatRelative(value: string): string {
    if (!value) {
      return 'Update time unavailable';
    }
    const epoch = toEpoch(value);
    if (!epoch) {
      return 'Update time unavailable';
    }
    const delta = Date.now() - epoch;
    if (delta < 60_000) {
      return 'Updated just now';
    }
    const minutes = Math.floor(delta / 60_000);
    if (minutes < 60) {
      return `Updated ${minutes}m ago`;
    }
    const hours = Math.floor(minutes / 60);
    if (hours < 24) {
      return `Updated ${hours}h ago`;
    }
    const days = Math.floor(hours / 24);
    return `Updated ${days}d ago`;
  }

  function formatTimestamp(value: string): string {
    if (!value) {
      return 'Unavailable';
    }
    const epoch = toEpoch(value);
    if (!epoch) {
      return 'Unavailable';
    }
    return new Date(epoch).toLocaleString();
  }

  function laneTitleFor(card: KanbanCard): string {
    const lane = KANBAN_LANES.find((entry) => entry.id === card.lane);
    return lane?.title || card.lane;
  }

  function dependenciesFor(card: KanbanCard): string[] {
    const raw = String(card.task?.metadata?.prd_story_dependencies || '').trim();
    if (!raw) {
      return [];
    }
    return raw.split(',').map((value) => value.trim()).filter(Boolean);
  }

  function loopIDForCard(card: KanbanCard): string {
    const taskLoop = String(card.task?.metadata?.loop_id || '').trim();
    return taskLoop;
  }

  function documentRefForCard(card: KanbanCard): string {
    const task = card.task;
    return String(task?.source_document || task?.metadata?.document_id || '').trim();
  }

  function acceptanceCriteriaFor(card: KanbanCard): string[] {
    return card.task?.acceptance_criteria || [];
  }

  function validationCommandsFor(card: KanbanCard): string[] {
    return card.task?.validation || [];
  }

</script>

<TopBar title="Tasks">
  {#snippet controls()}
    <div class="tasks-filters">
      <div class="tasks-filter-lane-wrap">
        <select
          class="smith-filter-control tasks-filter-lane"
          bind:value={laneFilter}
          data-testid="tasks-filter-lane"
          aria-label="Filter tasks by lane"
        >
          <option value="all">All Lanes</option>
          <option value="in_focus">In Focus</option>
          <option value="backlog">Scheduled</option>
          <option value="blocked">Blocked</option>
          <option value="done">Finished</option>
        </select>
      </div>

      <div class="tasks-filter-search-wrap">
        <input
          class="smith-filter-control tasks-filter-search"
          type="search"
          bind:value={searchQuery}
          placeholder="Filter tasks..."
          data-testid="tasks-filter-search"
          aria-label="Filter tasks"
        />
      </div>
    </div>
  {/snippet}
</TopBar>

<section class="tasks-kanban-page px-4 pb-8">
  {#if accessValidated}
    {#if error}
      <p class="error">{error}</p>
    {/if}

    <div class="kanban-grid">
      {#each lanes as lane}
        <section class="lane" data-testid={`lane-${lane.id}`}>
          <div class="lane-head">
            <h3>{lane.title}</h3>
            <span class="lane-count">{lane.tasks.length}</span>
          </div>

          {#if lane.tasks.length === 0}
            <p class="muted lane-empty" data-testid={`lane-empty-${lane.id}`}>{lane.empty}</p>
          {:else}
            <div class="task-list">
              {#each lane.tasks as card}
                <button
                  type="button"
                  class="task-card"
                  class:selected={selectedCardID === card.id}
                  data-testid={`task-card-${lane.id}`}
                  onclick={() => (selectedCardID = card.id)}
                >
                  <div class="task-top">
                    <div class="task-id">{card.id}</div>
                    <span class="status-pill status-{card.status}">{card.status}</span>
                  </div>
                  <div class="task-objective">{card.objective}</div>
                  <div class="task-meta">{card.projectID} · {card.providerProfileID}</div>
                  {#if lane.id === 'in_focus'}
                    <div class="task-step">{card.step}</div>
                    {#if card.runtimeState}
                      <div class="task-state">{card.runtimeState}</div>
                    {/if}
                  {/if}
                  <div class="task-updated">{formatRelative(card.updatedAt)}</div>
                </button>
              {/each}
            </div>
          {/if}
        </section>
      {/each}
    </div>

    {#if selectedCard}
      <section class="task-detail" data-testid="task-detail-panel">
        <div class="detail-head">
          <h3 class="detail-title">{selectedCard.objective}</h3>
          <button type="button" class="detail-close" onclick={() => (selectedCardID = '')}>Close</button>
        </div>

        <div class="detail-grid">
          <div class="detail-item"><span class="detail-label">Task</span><span class="detail-value">{selectedCard.id}</span></div>
          <div class="detail-item"><span class="detail-label">Status</span><span class="detail-value">{selectedCard.status}</span></div>
          <div class="detail-item"><span class="detail-label">Lane</span><span class="detail-value">{laneTitleFor(selectedCard)}</span></div>
          <div class="detail-item"><span class="detail-label">Project</span><span class="detail-value">{selectedCard.projectID}</span></div>
          <div class="detail-item"><span class="detail-label">Provider</span><span class="detail-value">{selectedCard.providerProfileID}</span></div>
          <div class="detail-item"><span class="detail-label">Loop</span><span class="detail-value">{loopIDForCard(selectedCard) || 'Unbound'}</span></div>
          <div class="detail-item detail-item-wide"><span class="detail-label">Document</span><span class="detail-value">{documentRefForCard(selectedCard) || 'Unavailable'}</span></div>
          <div class="detail-item"><span class="detail-label">Updated</span><span class="detail-value">{formatTimestamp(selectedCard.updatedAt)}</span></div>
          <div class="detail-item"><span class="detail-label">Created</span><span class="detail-value">{formatTimestamp(selectedCard.createdAt)}</span></div>
          <div class="detail-item detail-item-wide"><span class="detail-label">Dependencies</span><span class="detail-value">{dependenciesFor(selectedCard).length > 0 ? dependenciesFor(selectedCard).join(', ') : 'None'}</span></div>
        </div>

        <div class="detail-lists">
          <div class="detail-list">
            <h4>Acceptance Criteria</h4>
            {#if acceptanceCriteriaFor(selectedCard).length === 0}
              <p class="muted">No acceptance criteria recorded.</p>
            {:else}
              <ul>
                {#each acceptanceCriteriaFor(selectedCard) as criterion}
                  <li>{criterion}</li>
                {/each}
              </ul>
            {/if}
          </div>

          <div class="detail-list">
            <h4>Validation</h4>
            {#if validationCommandsFor(selectedCard).length === 0}
              <p class="muted">No validation commands recorded.</p>
            {:else}
              <ul>
                {#each validationCommandsFor(selectedCard) as command}
                  <li><code>{command}</code></li>
                {/each}
              </ul>
            {/if}
          </div>
        </div>
      </section>
    {/if}
  {/if}
</section>

<style>
  .tasks-kanban-page {
    display: grid;
    gap: 1rem;
    padding-top: 0.45rem;
  }

  .tasks-filters {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .tasks-filter-lane-wrap {
    width: 10.5rem;
  }

  .tasks-filter-search-wrap {
    width: 13rem;
  }

  .tasks-filter-lane,
  .tasks-filter-search {
    width: 100%;
  }

  .tasks-filter-lane {
    color: #d1d5db;
  }

  .tasks-filter-search {
    color: #f3f4f6;
  }

  .kanban-grid {
    display: grid;
    gap: 0.9rem;
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .lane {
    border: 1px solid var(--border-subtle);
    background: var(--surface-2);
    border-radius: 0.6rem;
    padding: 0.75rem;
    min-height: 16rem;
    box-shadow: var(--elevation-1), var(--inner-highlight);
  }

  .lane-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.7rem;
  }

  .lane-count {
    border: 1px solid var(--border-strong);
    border-radius: 999px;
    padding: 0.1rem 0.45rem;
    color: #64748b;
    font-size: 0.73rem;
    font-weight: 600;
  }

  .task-list {
    display: grid;
    gap: 0.55rem;
  }

  .task-card {
    border: 1px solid var(--border-subtle);
    background: var(--surface-1);
    border-radius: 0.5rem;
    padding: 0.65rem;
    display: grid;
    gap: 0.35rem;
    width: 100%;
    text-align: left;
    cursor: pointer;
    box-shadow: var(--elevation-1), var(--inner-highlight);
    transition: transform 130ms ease, box-shadow 140ms ease, border-color 120ms ease;
  }

  .task-card:hover {
    transform: translateY(-1px);
    box-shadow: var(--elevation-2);
  }

  .task-card.selected {
    border-color: rgba(56, 189, 248, 0.55);
    box-shadow: inset 0 0 0 1px rgba(56, 189, 248, 0.25), var(--elevation-2);
  }

  .task-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.45rem;
  }

  .task-id {
    color: #93c5fd;
    font-size: 0.72rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  }

  .task-objective {
    color: #0f172a;
    font-size: 0.9rem;
    line-height: 1.35;
    font-weight: 550;
  }

  .task-meta,
  .task-step,
  .task-state,
  .task-updated {
    color: #64748b;
    font-size: 0.74rem;
  }

  .task-step {
    color: #334155;
  }

  .status-pill {
    border-radius: 999px;
    border: 1px solid rgba(255, 255, 255, 0.12);
    padding: 0.12rem 0.46rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
    color: #334155;
  }

  .status-running {
    border-color: rgba(56, 189, 248, 0.45);
    color: #7dd3fc;
  }

  .status-completed {
    border-color: rgba(74, 222, 128, 0.45);
    color: #86efac;
  }

  .status-blocked {
    border-color: rgba(248, 113, 113, 0.45);
    color: #fca5a5;
  }

  .lane-empty {
    font-size: 0.82rem;
    border: 1px dashed var(--border-strong);
    border-radius: 0.45rem;
    padding: 0.65rem;
  }

  .error {
    color: #fca5a5;
    margin: 0;
  }

  .task-detail {
    border: 1px solid var(--border-subtle);
    border-radius: 0.6rem;
    background: var(--surface-2);
    padding: 0.9rem;
    display: grid;
    gap: 0.8rem;
    box-shadow: var(--elevation-2), var(--inner-highlight);
  }

  .detail-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.8rem;
  }

  .detail-title {
    margin: 0;
    color: #0f172a;
    font-size: 0.95rem;
    letter-spacing: 0.02em;
  }

  .detail-close {
    border: 1px solid var(--border-subtle);
    border-radius: 0.4rem;
    background: var(--surface-1);
    color: #475569;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    padding: 0.2rem 0.45rem;
    cursor: pointer;
  }

  .detail-grid {
    display: grid;
    gap: 0.5rem;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .detail-item {
    display: grid;
    gap: 0.1rem;
    border: 1px solid var(--border-subtle);
    border-radius: 0.45rem;
    padding: 0.4rem 0.55rem;
    background: var(--surface-1);
  }

  .detail-item-wide {
    grid-column: 1 / -1;
  }

  .detail-label {
    color: #64748b;
    font-size: 0.64rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .detail-value {
    color: #0f172a;
    font-size: 0.78rem;
    line-height: 1.35;
  }

  .detail-lists {
    display: grid;
    gap: 0.7rem;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .detail-list {
    border: 1px solid var(--border-subtle);
    border-radius: 0.45rem;
    padding: 0.55rem;
    background: var(--surface-1);
  }

  .detail-list h4 {
    margin: 0 0 0.45rem;
    color: #0f172a;
    font-size: 0.76rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .detail-list ul {
    margin: 0;
    padding-left: 1rem;
    color: #1e293b;
    font-size: 0.78rem;
    line-height: 1.4;
    display: grid;
    gap: 0.25rem;
  }

  .muted {
    color: #64748b;
  }

  h3 {
    margin: 0;
    color: #334155;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  :global(.dark .lane-count),
  :global(.dark .task-meta),
  :global(.dark .task-state),
  :global(.dark .task-updated),
  :global(.dark .detail-label),
  :global(.dark .muted) {
    color: #94a3b8;
  }

  :global(.dark .task-objective),
  :global(.dark .detail-title),
  :global(.dark .detail-value),
  :global(.dark .detail-list h4),
  :global(.dark .detail-list ul),
  :global(.dark h3) {
    color: #f8fafc;
  }

  :global(.dark .task-step) {
    color: #cbd5e1;
  }

  :global(.dark .status-pill) {
    color: #cbd5e1;
  }

  @media (max-width: 1100px) {
    .kanban-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 720px) {
    .kanban-grid {
      grid-template-columns: 1fr;
    }

    .tasks-filters {
      width: 100%;
      justify-content: flex-end;
      flex-wrap: wrap;
    }

    .tasks-filter-lane-wrap,
    .tasks-filter-search-wrap {
      width: 100%;
    }

    .detail-grid,
    .detail-lists {
      grid-template-columns: 1fr;
    }
  }
</style>
