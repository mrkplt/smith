<script lang="ts">
	import { goto } from '$app/navigation';
  import { Card } from 'flowbite-svelte';

	interface Props {
		loop: any;
		selected: boolean;
		onSelect: (id: string) => void;
	}

	let { loop, selected, onSelect }: Props = $props();
  
  const statusPillClass = $derived.by(() => {
    switch (loop.status) {
      case 'running':
        return 'bg-emerald-500/20 text-emerald-200 border border-emerald-400/50';
      case 'unresolved':
        return 'bg-amber-500/20 text-amber-100 border border-amber-400/50';
      case 'synced':
        return 'bg-cyan-500/20 text-cyan-100 border border-cyan-400/50';
      case 'flatline':
        return 'bg-rose-500/20 text-rose-100 border border-rose-400/50';
      case 'cancelled':
        return 'bg-slate-500/20 text-slate-200 border border-slate-400/40';
      default:
        return 'bg-gray-500/20 text-gray-200 border border-gray-400/40';
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

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div 
	class="pod-card-container group" 
  role="button"
  tabindex="0"
	onclick={() => goto(`/pod-view/${encodeURIComponent(loop.loopID)}`)}
>
  <Card class="bg-slate-900/40 border-gray-800 hover:border-cyan-500/50 transition-all cursor-pointer h-full backdrop-blur-sm p-4 {selected ? 'ring-1 ring-cyan-500 border-cyan-500' : ''}">
    <div class="flex flex-col h-full gap-3">
      <div class="flex justify-between items-start gap-2">
        <div class="truncate text-sm font-mono font-bold text-gray-200 group-hover:text-cyan-400 transition-colors" title={loop.displayTitle || loop.loopID}>
          {loop.displayTitle || loop.loopID}
        </div>
        <span class={`rounded-full text-[10px] px-2.5 py-0.5 font-semibold capitalize tracking-wide ${statusPillClass}`}>
          {statusLabel}
        </span>
      </div>

      <div class="truncate text-[10px] font-mono text-gray-500" title={loop.loopID}>
        {loop.loopID}
      </div>

      <div class="flex items-center gap-2 text-xs font-semibold text-gray-500 uppercase tracking-wider">
        <span class="w-2 h-2 rounded-full bg-gray-700"></span>
        {loop.project}
      </div>

      <div class="text-xs text-gray-400 line-clamp-2 min-h-[32px] leading-relaxed">
        {loop.reason || "No recent updates available"}
      </div>

      <div class="mt-auto pt-3 border-t border-gray-800/50 flex justify-between items-center text-[10px] font-mono text-gray-500">
        <div class="flex gap-3">
          <span>PROG <span class="text-gray-300 font-bold">{loop.currentCount || loop.attempt || 0}/{loop.targetCount || loop.attempt || 1}</span></span>
          <span>REV <span class="text-gray-300 font-bold">{loop.revision}</span></span>
        </div>
        <div class="opacity-0 group-hover:opacity-100 transition-opacity text-cyan-500 font-bold">
          VIEW &rarr;
        </div>
      </div>
    </div>
  </Card>
</div>

<style>
  :global(.pod-card-container .p-4) {
    padding: 1rem !important;
  }
</style>
