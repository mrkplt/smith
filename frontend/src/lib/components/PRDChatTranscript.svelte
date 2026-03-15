<script lang="ts">
  import type { PRDChatMessage } from '$lib/chat/prd-chat';

  interface Props {
    chatMessages: PRDChatMessage[];
    busy: boolean;
    starting: boolean;
  }

  let { chatMessages, busy, starting }: Props = $props();
</script>

<div class="flex-1 overflow-y-auto p-4 space-y-4 bg-black rounded-none border border-gray-800 mb-4 relative">
  {#each chatMessages as msg}
    {#if msg.type !== 'system' || (msg.text && !msg.final_prd_path)}
      <div class="flex {msg.type === 'user' ? 'justify-end' : 'justify-start'}">
        <div class="max-w-[80%] px-4 py-2 rounded-none text-sm {msg.type === 'user' ? 'bg-[#86BC25] text-black font-bold' : 'bg-slate-900 text-gray-200 border border-gray-800'}">
          <div style="white-space: pre-wrap;">{msg.text || msg.error || ''}</div>
        </div>
      </div>
    {/if}
  {:else}
    {#if starting}
      <div class="flex flex-col justify-center items-center h-full text-gray-500 gap-4">
        <div class="w-8 h-8 border-4 border-[#86BC25] border-t-transparent rounded-full animate-spin"></div>
        <span class="italic text-xs uppercase font-bold tracking-widest">Starting chat session...</span>
      </div>
    {:else}
      <div class="flex justify-center items-center h-full text-gray-500 italic">
        Ready for your prompt.
      </div>
    {/if}
  {/each}

  {#if busy && !starting}
    <div class="flex justify-start">
      <div class="bg-slate-900 text-gray-400 border border-gray-800 px-4 py-2 rounded-none flex items-center gap-3">
        <div class="w-4 h-4 border-2 border-[#86BC25] border-t-transparent rounded-full animate-spin"></div>
        <span class="text-[10px] font-bold uppercase tracking-widest">Assistant is thinking...</span>
      </div>
    </div>
  {/if}
</div>
