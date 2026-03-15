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

<div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8 px-4">
  <div class="bg-slate-900/20 border border-gray-900 rounded-none p-5 flex flex-col gap-1 backdrop-blur-sm">
    <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Total Pods</span>
    <span class="text-3xl font-bold text-white tracking-tighter">{stats.total}</span>
  </div>
  <div class="bg-slate-900/20 border border-gray-900 rounded-none p-5 flex flex-col gap-1 backdrop-blur-sm">
    <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Active Loops</span>
    <span class="text-3xl font-bold text-[#86BC25] tracking-tighter">{stats.active}</span>
  </div>
  <div class="bg-slate-900/20 border border-gray-900 rounded-none p-5 flex flex-col gap-1 backdrop-blur-sm">
    <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Flatline</span>
    <span class="text-3xl font-bold text-rose-600 tracking-tighter">{stats.flatline}</span>
  </div>
</div>
