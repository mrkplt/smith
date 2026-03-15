<script lang="ts">
  import { BrainOutline } from 'flowbite-svelte-icons';

  interface Props {
    providerLabel?: string;
  }

  let { providerLabel = 'OpenAI Codex' }: Props = $props();
</script>

<div class="bg-slate-900/30 p-4 border border-gray-800 rounded-none">
  <div class="flex items-center gap-3 mb-2 text-[#86BC25]">
    <BrainOutline size="sm" />
    <span class="text-[10px] font-bold uppercase tracking-widest">OpenAI Integration</span>
  </div>
  <p class="text-[11px] text-gray-400 leading-relaxed">
    Configure your OpenAI API key to enable autonomous code generation and analysis for {providerLabel}. Keys are securely stored and never exposed in the UI.
  </p>
</div>
