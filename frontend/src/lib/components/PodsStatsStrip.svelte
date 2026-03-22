<script lang="ts">
  interface Props {
    stats: {
      total: number;
      active: number;
      flatline: number;
    };
  }

  let { stats }: Props = $props();
</script>

<div class="pods-stats-strip grid grid-cols-1 md:grid-cols-3 gap-6">
  <div class="stat-card">
    <span class="stat-label">Total Pods</span>
    <span class="stat-value">{stats.total}</span>
  </div>

  <div class="stat-card">
    <span class="stat-label">Active Loops</span>
    <span class="stat-value stat-value-active">{stats.active}</span>
  </div>

  <div class="stat-card">
    <span class="stat-label">Flatline</span>
    <span class="stat-value stat-value-flatline">{stats.flatline}</span>
  </div>
</div>

<style>
  .pods-stats-strip {
    margin: 0;
  }

  .stat-card {
    border: 1px solid var(--border-subtle);
    background: var(--surface-2);
    padding: 1.1rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    box-shadow: var(--elevation-1), var(--inner-highlight);
  }

  .stat-label {
    font-size: 0.68rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.2em;
    color: #64748b;
  }

  .stat-value {
    font-size: 2.2rem;
    line-height: 1;
    font-weight: 700;
    letter-spacing: -0.03em;
    color: #1e293b;
  }

  .stat-value-active {
    color: #65a30d;
  }

  .stat-value-flatline {
    color: #ef4444;
  }

  :global(.dark .stat-label) {
    color: #64748b;
  }

  :global(.dark .stat-value) {
    color: #f8fafc;
  }
</style>
