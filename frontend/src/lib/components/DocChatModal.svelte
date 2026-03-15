<script lang="ts">
	import { onDestroy } from 'svelte';
  import { Modal, Button, Badge } from 'flowbite-svelte';
  import PRDChatTranscript from '$lib/components/PRDChatTranscript.svelte';
  import PRDChatComposer from '$lib/components/PRDChatComposer.svelte';
  import { connectPRDChat, sendPRDChatMessage, type PRDChatMessage } from '$lib/chat/prd-chat';

	interface Props {
		open: boolean;
		onClose: () => void;
		onDraftFinalized: (title: string, content: string) => void;
	}

	let { open = $bindable(), onClose, onDraftFinalized }: Props = $props();

	let chatMessages = $state<PRDChatMessage[]>([]);
	let chatSocket = $state<WebSocket | null>(null);
	let chatInput = $state('');
	let busy = $state(false);
	let starting = $state(false);
	let finalContent = $state<string | null>(null);
	let finalTitle = $state<string | null>(null);

	function connectChat() {
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
		chatSocket = connectPRDChat((next) => {
			if (next.messages) {
				chatMessages = [...chatMessages, ...next.messages];
			}
			if (next.finalContent !== undefined) {
				finalContent = next.finalContent;
			}
			if (next.finalTitle !== undefined) {
				finalTitle = next.finalTitle;
			}
			if (next.busy !== undefined) {
				busy = next.busy;
			}
			if (next.starting !== undefined) {
				starting = next.starting;
			}
		});
	}

	function sendChatMessage() {
		if (!sendPRDChatMessage(chatSocket, chatInput)) return;
		chatMessages = [...chatMessages, { type: 'user', text: chatInput }];
		chatInput = '';
		busy = true;
	}

	function handleFinalize() {
		if (finalContent) {
			onDraftFinalized(finalTitle || "New Document", finalContent);
			onClose();
		}
	}

	$effect(() => {
		if (open) {
			connectChat();
		} else {
			if (chatSocket) {
				chatSocket.close();
				chatSocket = null;
			}
		}
	});

	onDestroy(() => {
		if (chatSocket) chatSocket.close();
	});
</script>

<Modal bind:open title="Draft Document with AI" size="lg" autoclose={false} class="bg-black border border-gray-800 rounded-none">
  <div class="flex flex-col h-[500px]">
    <PRDChatTranscript
      {chatMessages}
      {busy}
      {starting}
    />

    {#if finalContent}
      <div class="p-3 bg-[#86BC25]/10 border border-[#86BC25]/30 rounded-none mb-4 flex justify-between items-center">
        <Badge color="green" class="bg-[#86BC25] text-black font-bold">Document Ready</Badge>
        <Button size="xs" color="alternative" class="bg-[#86BC25] text-black font-bold px-4 py-1" onclick={handleFinalize}>Review & Save</Button>
      </div>
    {/if}

    <PRDChatComposer
      {chatInput}
      disabled={!chatSocket || starting}
      onChatInputChange={(value) => chatInput = value}
      onSend={sendChatMessage}
    />
  </div>

  <svelte:fragment slot="footer">
    <Button color="alternative" class="rounded-none border-gray-700" onclick={onClose}>Close</Button>
  </svelte:fragment>
</Modal>
