<script lang="ts">
	import { onDestroy } from 'svelte';
  import { Modal, Button, Badge } from 'flowbite-svelte';
  import PRDChatTranscript from '$lib/components/PRDChatTranscript.svelte';
  import PRDChatComposer from '$lib/components/PRDChatComposer.svelte';
  import { connectPRDChat, sendPRDChatMessage, type PRDChatMessage, type PRDChatSocket } from '$lib/chat/prd-chat';

	interface Props {
		open: boolean;
		onClose: () => void;
		onDraftFinalized: (title: string, content: string) => void;
	}

	let { open = $bindable(), onClose, onDraftFinalized }: Props = $props();

	let chatMessages = $state<PRDChatMessage[]>([]);
	let chatSocket = $state<PRDChatSocket | null>(null);
	let chatInput = $state('');
	let busy = $state(false);
	let starting = $state(false);
	let finalContent = $state<string | null>(null);
	let finalTitle = $state<string | null>(null);
	let chatProvider = $state('');
	let chatThinkingLevel = $state('balanced');
	let chatSettingsDirty = $state(false);

	function connectChat() {
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}

		const providerApiKey = typeof window !== 'undefined'
			? (window.localStorage.getItem('smith.chat.providerApiKey') || '').trim()
			: '';
		const context: Record<string, string> = {};
		if (chatProvider.trim() !== '') {
			context.provider = chatProvider.trim();
		}
		const resolvedModel = modelForThinking(chatProvider, chatThinkingLevel);
		if (resolvedModel !== '') {
			context.model = resolvedModel;
		}
		context.thinkingLevel = chatThinkingLevel;
		if (providerApiKey !== '') {
			context.providerApiKey = providerApiKey;
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
		}, { context });
		chatSettingsDirty = false;
	}

	function modelForThinking(activeProvider: string, level: string): string {
		const normalizedProvider = activeProvider.trim().toLowerCase();
		if (normalizedProvider !== '' && normalizedProvider !== 'openai') {
			return '';
		}
		return 'gpt-5.4';
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
			if (typeof window !== 'undefined') {
				if (chatProvider.trim() === '') {
					chatProvider = window.localStorage.getItem('smith.chat.provider') || '';
				}
				const savedThinking = window.localStorage.getItem('smith.chat.thinkingLevel');
				if (savedThinking === 'quick' || savedThinking === 'balanced' || savedThinking === 'deep') {
					chatThinkingLevel = savedThinking;
				}
			}
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
		<div class="grid grid-cols-1 md:grid-cols-4 gap-2 mb-3">
			<select
				class="bg-slate-900 border border-gray-800 text-white text-xs rounded-none px-2 py-2"
				value={chatProvider}
				oninput={(event) => {
					chatProvider = (event.currentTarget as HTMLSelectElement).value;
					chatSettingsDirty = true;
				}}
			>
				<option value="">Service default provider</option>
				<option value="openai">openai</option>
				<option value="anthropic">anthropic</option>
				<option value="google">google</option>
			</select>
			<select
				class="md:col-span-2 bg-slate-900 border border-gray-800 text-white text-xs rounded-none px-2 py-2"
				value={chatThinkingLevel}
				oninput={(event) => {
					chatThinkingLevel = (event.currentTarget as HTMLSelectElement).value;
					chatSettingsDirty = true;
				}}
			>
				<option value="quick">Thinking: quick</option>
				<option value="balanced">Thinking: balanced</option>
				<option value="deep">Thinking: deep</option>
			</select>
			<Button color="alternative" size="sm" class="rounded-none border-gray-700 text-xs" onclick={connectChat}>
				{chatSettingsDirty ? 'Apply settings' : 'Restart chat'}
			</Button>
		</div>

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
