<script lang="ts">
  import { Badge, Button, Input } from 'flowbite-svelte';
  import { PaperPlaneOutline } from 'flowbite-svelte-icons';
  import type { PRDChatSocket } from '$lib/chat/prd-chat';

  interface Props {
    chatMessages: { type: string; text?: string; error?: string; final_prd_path?: string }[];
    finalPRD: string | null;
    chatSocket: PRDChatSocket | null;
    chatInput: string;
    chatProvider: string;
    chatThinkingLevel: string;
    onChatInputChange: (value: string) => void;
    onChatProviderChange: (value: string) => void;
    onChatThinkingLevelChange: (value: string) => void;
    onSendChatMessage: () => void;
    onRestartChat: () => void;
  }

  let {
    chatMessages,
    finalPRD,
    chatSocket,
    chatInput,
    chatProvider,
    chatThinkingLevel,
    onChatInputChange,
    onChatProviderChange,
    onChatThinkingLevelChange,
    onSendChatMessage,
    onRestartChat
  }: Props = $props();
</script>

<div class="flex flex-col h-[400px]">
  <div class="mb-3 grid grid-cols-1 md:grid-cols-3 gap-2">
    <select
      class="bg-slate-900 border border-gray-800 text-white text-xs rounded-none px-2 py-2"
      value={chatProvider}
      oninput={(event) => onChatProviderChange((event.currentTarget as HTMLSelectElement).value)}
    >
      <option value="">Use service default provider</option>
      <option value="openai">openai</option>
      <option value="anthropic">anthropic</option>
      <option value="google">google</option>
    </select>
    <select
      class="bg-slate-900 border border-gray-800 text-white text-xs rounded-none px-2 py-2"
      value={chatThinkingLevel}
      oninput={(event) => onChatThinkingLevelChange((event.currentTarget as HTMLSelectElement).value)}
    >
      <option value="quick">Thinking: quick</option>
      <option value="balanced">Thinking: balanced</option>
      <option value="deep">Thinking: deep</option>
    </select>
    <Button color="alternative" class="rounded-none border-gray-700 text-xs" onclick={onRestartChat}>
      Restart with settings
    </Button>
  </div>

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
        Starting chat session...
      </div>
    {/each}
  </div>

  {#if finalPRD}
    <Badge color="green" class="mb-4 py-2 rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px]">Draft Ready</Badge>
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
