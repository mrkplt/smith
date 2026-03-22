<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import TopBar from '$lib/components/TopBar.svelte';
  import { isTasksKanbanVisible } from '$lib/feature-flags';
  import { pushToast } from '$lib/stores';

  let accessValidated = $state(false);

  onMount(async () => {
    if (!isTasksKanbanVisible()) {
      console.info('[feature-flag] feature=tasks-kanban action=route-block redirect=/tasks reason=disabled');
      pushToast('Tasks Kanban is not enabled in this environment', 'muted');
      await goto('/tasks', { replaceState: true });
      return;
    }
    accessValidated = true;
  });
</script>

<TopBar title="Tasks Kanban" />

<section class="tasks-kanban-page px-4 pb-8">
  {#if accessValidated}
    <div class="panel">
      <h2>Tasks Kanban</h2>
      <p class="muted">Kanban workspace is enabled for this environment.</p>
    </div>
  {/if}
</section>

<style>
  .tasks-kanban-page {
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

  .muted {
    color: #9ca3af;
  }
</style>
