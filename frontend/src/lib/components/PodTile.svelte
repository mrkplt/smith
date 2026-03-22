<script lang="ts">
	import { goto } from '$app/navigation';

	interface Props {
		loop: any;
		selected: boolean;
		onSelect: (id: string) => void;
	}

	let { loop, selected, onSelect }: Props = $props();
  
  const statusPillClass = $derived.by(() => {
    switch (loop.status) {
      case 'running':
        return 'status-running';
      case 'unresolved':
        return 'status-unresolved';
      case 'synced':
        return 'status-synced';
      case 'flatline':
        return 'status-flatline';
      case 'cancelled':
        return 'status-cancelled';
      default:
        return 'status-default';
    }
  });

  const statusLabel = $derived.by(() => {
    switch (loop.status) {
      case 'flatline':
        return 'flatlined';
      default:
        return String(loop.status || 'unknown');
    }
  });
</script>

<button
	type="button"
	class={`pod-card ${selected ? 'selected' : ''}`}
	onclick={() => goto(`/pod-view/${encodeURIComponent(loop.loopID)}`)}
>
	<div class="pod-content">
		<div class="pod-head">
			<div class="pod-title" title={loop.displayTitle || loop.loopID}>{loop.displayTitle || loop.loopID}</div>
			<span class={`pod-status ${statusPillClass}`}>{statusLabel}</span>
		</div>

		<div class="pod-id" title={loop.loopID}>{loop.loopID}</div>

		<div class="pod-project"><span class="pod-project-dot"></span>{loop.project}</div>

		<div class="pod-reason">{loop.reason || 'No recent updates available'}</div>

		<div class="pod-meta-row">
			<div class="pod-meta-values">
				<span>POD: <strong>{loop.currentCount || loop.attempt || 0}/{loop.targetCount || loop.attempt || 1}</strong></span>
				<span>REV <strong>{loop.revision}</strong></span>
			</div>
			<div class="pod-view">View</div>
		</div>
	</div>
</button>

<style>
  .pod-card {
    appearance: none;
    -webkit-appearance: none;
    font: inherit;
    line-height: 1.2;
    white-space: normal;
    width: 100%;
    border: 1px solid var(--border-subtle);
    background: var(--surface-2);
    border-radius: 0.5rem;
    text-align: left;
    padding: 1rem;
    display: block;
    cursor: pointer;
    overflow: hidden;
    box-shadow: var(--elevation-1), var(--inner-highlight);
    transition: transform 140ms ease, border-color 120ms ease, box-shadow 150ms ease;
  }

  .pod-card:hover {
    transform: translateY(-2px);
    border-color: rgba(59, 130, 246, 0.5);
    box-shadow: var(--elevation-3);
  }

  .pod-card.selected {
    border-color: rgba(59, 130, 246, 0.7);
    box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.35), var(--elevation-2);
  }

  .pod-content {
    display: grid;
    gap: 0.6rem;
    min-width: 0;
  }

  .pod-head {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    align-items: flex-start;
    min-width: 0;
  }

  .pod-title {
    font-size: 0.98rem;
    line-height: 1.2;
    font-family: var(--mono);
    font-weight: 700;
    color: #243b61;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .pod-status {
    border-radius: 999px;
    font-size: 0.66rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 0.16rem 0.52rem;
    font-weight: 700;
    flex: 0 0 auto;
  }

  .pod-id {
    color: #64748b;
    font-size: 0.7rem;
    font-family: var(--mono);
    line-clamp: 1;
    display: -webkit-box;
    -webkit-line-clamp: 1;
    -webkit-box-orient: vertical;
    overflow: hidden;
    overflow-wrap: anywhere;
  }

  .status-running {
    background: rgba(34, 197, 94, 0.18);
    border: 1px solid rgba(34, 197, 94, 0.5);
    color: #15803d;
  }

  .status-unresolved {
    background: rgba(245, 158, 11, 0.18);
    border: 1px solid rgba(245, 158, 11, 0.5);
    color: #b45309;
  }

  .status-synced {
    background: rgba(59, 130, 246, 0.18);
    border: 1px solid rgba(59, 130, 246, 0.5);
    color: #1d4ed8;
  }

  .status-flatline {
    background: rgba(239, 68, 68, 0.18);
    border: 1px solid rgba(239, 68, 68, 0.5);
    color: #b91c1c;
  }

  .status-cancelled,
  .status-default {
    background: rgba(148, 163, 184, 0.2);
    border: 1px solid rgba(148, 163, 184, 0.45);
    color: #475569;
  }

  .pod-project {
    display: flex;
    align-items: center;
    gap: 0.42rem;
    font-size: 0.8rem;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: #64748b;
    font-weight: 700;
  }

  .pod-project-dot {
    width: 0.36rem;
    height: 0.36rem;
    border-radius: 999px;
    background: #64748b;
  }

  .pod-reason {
    min-height: 2.5rem;
    font-size: 0.82rem;
    line-height: 1.4;
    color: #334155;
    line-clamp: 2;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    overflow-wrap: anywhere;
  }

  .pod-meta-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-top: 1px solid rgba(148, 163, 184, 0.35);
    padding-top: 0.65rem;
  }

  .pod-meta-values {
    display: flex;
    gap: 0.85rem;
    color: #64748b;
    font-family: var(--mono);
    font-size: 0.68rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .pod-meta-values span {
    white-space: nowrap;
  }

  .pod-meta-values strong {
    color: #243b61;
  }

  .pod-view {
    color: #3b82f6;
    text-transform: uppercase;
    font-size: 0.66rem;
    letter-spacing: 0.08em;
    font-weight: 700;
    opacity: 0;
    transition: opacity 120ms ease;
  }

  .pod-card:hover .pod-view {
    opacity: 1;
  }

  :global(.dark .pod-title) {
    color: #e5e7eb;
  }

  :global(.dark .pod-id),
  :global(.dark .pod-project),
  :global(.dark .pod-reason),
  :global(.dark .pod-meta-values) {
    color: #94a3b8;
  }

  :global(.dark .pod-project-dot) {
    background: #475569;
  }

  :global(.dark .pod-meta-values strong) {
    color: #d1d5db;
  }

  :global(.dark .pod-meta-row) {
    border-top-color: rgba(31, 41, 55, 0.65);
  }

  :global(.dark .status-running) {
    background: rgba(16, 185, 129, 0.18);
    border-color: rgba(16, 185, 129, 0.45);
    color: #a7f3d0;
  }

  :global(.dark .status-unresolved) {
    background: rgba(245, 158, 11, 0.18);
    border-color: rgba(245, 158, 11, 0.45);
    color: #fcd34d;
  }

  :global(.dark .status-synced) {
    background: rgba(59, 130, 246, 0.18);
    border-color: rgba(59, 130, 246, 0.45);
    color: #bfdbfe;
  }

  :global(.dark .status-flatline) {
    background: rgba(244, 63, 94, 0.18);
    border-color: rgba(244, 63, 94, 0.45);
    color: #fecdd3;
  }

  :global(.dark .status-cancelled),
  :global(.dark .status-default) {
    background: rgba(71, 85, 105, 0.2);
    border-color: rgba(148, 163, 184, 0.45);
    color: #e2e8f0;
  }
</style>
