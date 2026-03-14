<script lang="ts">
  import { Badge, Button, Input } from 'flowbite-svelte';
  import { PaperPlaneOutline } from 'flowbite-svelte-icons';

  interface Props {
    chatMessages: { type: string; text?: string; error?: string; final_prd_path?: string }[];
    finalPRD: string | null;
    chatSocket: WebSocket | null;
    chatInput: string;
    onChatInputChange: (value: string) => void;
    onSendChatMessage: () => void;
  }

  let { chatMessages, finalPRD, chatSocket, chatInput, onChatInputChange, onSendChatMessage }: Props = $props();
</script>

<div class="flex flex-col h-[400px]">
  <div class="flex-1 overflow-y-auto p-4 space-y-4 bg-black rounded-none border border-gray-800 mb-4">
    {#each chatMessages as msg}
      {#if msg.type !== 'system' || msg.text}
        <div class="flex {msg.type === 'user' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-[85%] px-3 py-2 rounded-none text-xs {msg.type === 'user' ? 'bg-[#86BC25] text-black font-bold' : 'bg-slate-900 text-gray-200 border border-gray-800'}">
            <div style="white-space: pre-wrap;">{msg.text || msg.error || ""}</div>
          </div>
        </div>
      {/if}
    {:else}
      <div class="flex justify-center items-center h-full text-gray-500 italic text-sm">
        Initializing PRD chat...
      </div>
    {/each}
  </div>

  {#if finalPRD}
    <Badge color="green" class="mb-4 py-2 rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px]">PRD Finalized</Badge>
  {/if}

  <div class="flex gap-2">
    <Input
      type="text"
      placeholder="Refine requirements..."
      value={chatInput}
      oninput={(event) => onChatInputChange((event.currentTarget as HTMLInputElement).value)}
      disabled={!chatSocket}
      onkeydown={(event) => event.key === 'Enter' && onSendChatMessage()}
      class="bg-slate-900 border-gray-800 text-white rounded-none"
    />
    <Button color="alternative" class="bg-[#86BC25] text-black px-4 rounded-none" onclick={onSendChatMessage} disabled={!chatSocket || !chatInput}>
      <PaperPlaneOutline size="sm" />
    </Button>
  </div>
</div>
