<script lang="ts">
  interface Props {
    method: string;
    onSelectMethod: (method: string) => void;
  }

  let { method, onSelectMethod }: Props = $props();
</script>

<div class="space-y-4">
  <p class="text-gray-400 text-sm">Choose how you want to start this development loop.</p>
  <div class="grid grid-cols-1 gap-3">
    <button
      class="p-4 rounded-none border text-left transition-all {method === 'issue' ? 'bg-[#86BC25]/10 border-[#86BC25] text-[#86BC25]' : 'bg-slate-900 border-gray-800 text-gray-400 hover:border-gray-600'}"
      onclick={() => onSelectMethod('issue')}
    >
      <div class="font-bold uppercase text-xs tracking-widest">Create from Issue</div>
      <div class="text-[10px] opacity-70 mt-1 uppercase">Import requirements from a GitHub issue.</div>
    </button>
    <button
      class="p-4 rounded-none border text-left transition-all {method === 'generate_prd' ? 'bg-[#86BC25]/10 border-[#86BC25] text-[#86BC25]' : 'bg-slate-900 border-gray-800 text-gray-400 hover:border-gray-600'}"
      onclick={() => onSelectMethod('generate_prd')}
    >
      <div class="font-bold uppercase text-xs tracking-widest">Generate a PRD</div>
      <div class="text-[10px] opacity-70 mt-1 uppercase">Chat with an agent to build a new document.</div>
    </button>
    <button
      class="p-4 rounded-none border text-left transition-all {method === 'load_prd' ? 'bg-[#86BC25]/10 border-[#86BC25] text-[#86BC25]' : 'bg-slate-900 border-gray-800 text-gray-400 hover:border-gray-600'}"
      onclick={() => onSelectMethod('load_prd')}
    >
      <div class="font-bold uppercase text-xs tracking-widest">Load a PRD</div>
      <div class="text-[10px] opacity-70 mt-1 uppercase">Paste raw JSON or a manual prompt.</div>
    </button>
  </div>
</div>
