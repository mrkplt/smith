<script lang="ts">
	import { onDestroy } from 'svelte';
  import { Button, Badge } from 'flowbite-svelte';
  import { CloseOutline } from 'flowbite-svelte-icons';
  import PRDChatTranscript from '$lib/components/PRDChatTranscript.svelte';
  import PRDChatComposer from '$lib/components/PRDChatComposer.svelte';
  import type { PRDValidationReport } from '$lib/documents/prd-validation';
  import { fetchJSON } from '$lib/api';
  import { loadChatSettings, resolveDefaultModel, resolveProviderType } from '$lib/chat/defaults';
  import { isProviderTypeEnabled } from '$lib/feature-flags';
  import { connectPRDChat, sendPRDChatMessage, type DocumentPatchProposal, type PRDChatMessage, type PRDChatSocket } from '$lib/chat/prd-chat';

	interface PRDDraftSeed {
		title: string;
		content: string;
		format: 'markdown' | 'json';
		validationReport: PRDValidationReport | null;
		targetedDiagnostic?: {
			code: string;
			message: string;
			suggestion?: string;
			path?: string;
			storyId?: string;
		};
		documentId?: string;
		documentVersion?: string;
		documentStatus?: string;
		projectId?: string;
	}

	interface Props {
		open: boolean;
		onClose: () => void;
		onDraftFinalized: (title: string, content: string) => void;
    seedDraft?: PRDDraftSeed | null;
		focusContext?: Record<string, unknown> | null;
	}

	let { open = $bindable(), onClose, onDraftFinalized, seedDraft = null, focusContext = null }: Props = $props();

	let chatMessages = $state<PRDChatMessage[]>([]);
	let chatSocket = $state<PRDChatSocket | null>(null);
	let chatInput = $state('');
	let busy = $state(false);
	let starting = $state(false);
	let assistantDraft = $state<string | null>(null);
	let finalContent = $state<string | null>(null);
	let finalTitle = $state<string | null>(null);
	let readinessStatus = $state<string | null>(null);
	let sessionIntent = $state<string | null>(null);
	let documentVersion = $state<string | null>(null);
	let contextDrift = $state(false);
	let patchProposal = $state<DocumentPatchProposal | null>(null);
	let chatProviderProfiles = $state<any[]>([]);
	let chatProviderProfileID = $state('');
	let chatThinkingLevel = $state('balanced');
	let lastFocusSignature = $state('');

	async function loadProviderProfiles() {
		try {
			const profiles = await fetchJSON('/v1/providers');
			chatProviderProfiles = Array.isArray(profiles)
				? profiles.filter((profile) => isProviderTypeEnabled(String(profile?.provider_type || profile?.id || '')))
				: [];
		} catch {
			chatProviderProfiles = [];
		}
	}

	function applyStoredChatSettings() {
		if (typeof window === 'undefined') {
			return;
		}
		const settings = loadChatSettings(window.localStorage, chatProviderProfiles);
		chatProviderProfileID = settings.providerProfileID;
		chatThinkingLevel = settings.thinkingLevel;
	}

	function chooseProviderProfile(nextProfileID: string) {
		chatProviderProfileID = nextProfileID;
	}

	function handleComposerProviderChange(nextProfileID: string) {
		if (nextProfileID === chatProviderProfileID) {
			return;
		}
		chooseProviderProfile(nextProfileID);
		connectChat();
	}

	function handleComposerThinkingLevelChange(nextThinkingLevel: string) {
		if (nextThinkingLevel === chatThinkingLevel) {
			return;
		}
		chatThinkingLevel = nextThinkingLevel;
		connectChat();
	}

	function connectChat() {
		chatMessages = [];
		assistantDraft = null;
		finalContent = null;
		finalTitle = null;
		readinessStatus = null;
		sessionIntent = null;
		documentVersion = null;
		contextDrift = false;
		patchProposal = null;
		busy = false;
		starting = false;
		lastFocusSignature = '';

		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}

		const providerApiKey = typeof window !== 'undefined'
			? (window.localStorage.getItem('smith.chat.providerApiKey') || '').trim()
			: '';
		const context: Record<string, string> = {};
		context.app = 'smith-console';
		context.surface = 'documents';
		context.route = '/documents';
		if (seedDraft?.projectId) {
			context.projectId = seedDraft.projectId;
		}
		if (seedDraft?.documentId) {
			context.documentId = seedDraft.documentId;
		}
		if (seedDraft?.documentVersion) {
			context.documentVersion = seedDraft.documentVersion;
		}
		if (seedDraft?.documentStatus) {
			context.documentStatus = seedDraft.documentStatus;
		}
		if (seedDraft?.title) {
			context.documentTitle = seedDraft.title;
		}
		if (seedDraft?.format) {
			context.documentFormat = seedDraft.format;
		}
		if (seedDraft?.validationReport?.readiness) {
			context.readinessStatus = seedDraft.validationReport.readiness;
			const diagnostics = [...seedDraft.validationReport.errors, ...seedDraft.validationReport.warnings].slice(0, 8);
			if (diagnostics.length > 0) {
				context.readinessDiagnostics = JSON.stringify(diagnostics);
			}
		}
		context.sessionIntent = 'document_refinement';
		const resolvedProvider = resolveProviderType(chatProviderProfiles, chatProviderProfileID, '');
		context.provider = resolvedProvider !== '' ? resolvedProvider : 'openai';
		if (chatProviderProfileID.trim() !== '') {
			context.providerProfileID = chatProviderProfileID.trim();
		}
		let resolvedModel = resolveDefaultModel(chatProviderProfiles, chatProviderProfileID, '');
		if (resolvedModel === '') {
			resolvedModel = modelForThinking(resolvedProvider, chatThinkingLevel);
		}
		if (resolvedModel !== '') {
			context.model = resolvedModel;
		}
		context.thinkingLevel = chatThinkingLevel;
		if (providerApiKey !== '') {
			context.providerApiKey = providerApiKey;
		}

		const initialPrompt = buildInitialPrompt(seedDraft);

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
			if (next.patchProposal !== undefined) {
				patchProposal = next.patchProposal;
			}
			if (next.assistantDraft !== undefined) {
				assistantDraft = next.assistantDraft;
			}
			if (next.readinessStatus !== undefined) {
				readinessStatus = next.readinessStatus;
			}
			if (next.sessionIntent !== undefined) {
				sessionIntent = next.sessionIntent;
			}
			if (next.documentVersion !== undefined) {
				documentVersion = next.documentVersion;
			}
			if (next.contextDrift !== undefined) {
				contextDrift = next.contextDrift;
			}
			if (next.busy !== undefined) {
				busy = next.busy;
			}
			if (next.starting !== undefined) {
				starting = next.starting;
			}
		}, { context, initialPrompt, sessionIntent: 'document_refinement' });

		syncFocusContext();
	}

	function syncFocusContext() {
		if (!chatSocket || !focusContext) {
			return;
		}
		const signature = JSON.stringify(focusContext);
		if (signature === '' || signature === lastFocusSignature) {
			return;
		}
		lastFocusSignature = signature;
		chatSocket.updateContext({
			type: 'ui.context.updated',
			focusContext,
		});
	}

	function buildInitialPrompt(seed: PRDDraftSeed | null): string | undefined {
		if (!seed || seed.content.trim() === '') {
			return undefined;
		}

		const diagnostics = seed.validationReport
			? [...seed.validationReport.errors, ...seed.validationReport.warnings]
			: [];
		const highlighted = diagnostics
			.slice(0, 8)
			.map((item, index) => {
				const suggestion = item.suggestion ? ` Suggestion: ${item.suggestion}` : '';
				return `${index + 1}. [${item.code}] ${item.message}.${suggestion}`;
			})
			.join('\n');

		const truncatedContent = seed.content.length > 12000
			? `${seed.content.slice(0, 12000)}\n\n[truncated for prompt length]`
			: seed.content;

		return [
			`I have an existing PRD titled "${seed.title || 'Untitled PRD'}" in ${seed.format.toUpperCase()} format.`,
			seed.targetedDiagnostic
				? `Resolve this specific readiness issue with minimal, precise edits while preserving intent: [${seed.targetedDiagnostic.code}] ${seed.targetedDiagnostic.message}`
				: 'Help me improve it so it satisfies Smith PRD readiness validation criteria and keeps the intent intact.',
			seed.targetedDiagnostic?.suggestion
				? `Diagnostic suggestion: ${seed.targetedDiagnostic.suggestion}`
				: '',
			highlighted !== ''
				? `Current validation findings:\n${highlighted}`
				: 'Current validation findings: none yet, but please proactively strengthen quality gates and acceptance criteria.',
			'When proposing finalized edits, include a `document_patch_proposal` JSON object with a `replace_document` operation containing the full revised markdown.',
			'Current PRD:',
			'```',
			truncatedContent,
			'```'
		].join('\n\n');
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

	function rejectPatchProposal() {
		patchProposal = null;
		finalContent = null;
		finalTitle = null;
	}

	function readinessBadgeColor(status: string | null): 'green' | 'yellow' | 'red' | 'gray' {
		if (status === 'pass') return 'green';
		if (status === 'warn') return 'yellow';
		if (status === 'fail') return 'red';
		return 'gray';
	}

	$effect(() => {
		if (!open) {
			if (chatSocket) {
				chatSocket.close();
				chatSocket = null;
			}
			return;
		}

		void loadProviderProfiles().then(() => {
			applyStoredChatSettings();
			connectChat();
		});
	});

	$effect(() => {
		focusContext;
		if (!open) {
			return;
		}
		syncFocusContext();
	});

	onDestroy(() => {
		if (chatSocket) chatSocket.close();
	});
</script>

<aside id="documents-prd-chat-drawer" class="prd-chat-drawer">
	<div class="flex flex-col h-full bg-black border-l border-gray-800 shadow-2xl">
		<div class="px-8 py-6 border-b border-gray-900 bg-slate-900/20 flex items-center justify-between">
			<div>
				<h2 class="text-xl font-bold text-white uppercase tracking-tight">PRD Draft & Refinement</h2>
				<p class="text-[10px] font-bold text-gray-500 uppercase tracking-[0.2em] mt-1">Right-side assistant drawer</p>
				<div class="mt-2 flex flex-wrap items-center gap-2 text-[10px] uppercase tracking-[0.14em]">
					{#if seedDraft?.title}
						<span class="text-gray-400">Doc: {seedDraft.title}</span>
					{/if}
					{#if documentVersion}
						<span class="text-gray-500">Version: {documentVersion}</span>
					{/if}
					{#if sessionIntent}
						<span class="text-gray-500">Intent: {sessionIntent}</span>
					{/if}
				</div>
			</div>
			<button class="text-white hover:text-[#86BC25] transition-colors p-2" onclick={onClose} aria-label="Close PRD drawer">
				<CloseOutline size="md" />
			</button>
		</div>

		<div class="flex-1 min-h-0 p-6 flex flex-col">
			<div class="mb-4 flex flex-wrap items-center gap-2">
				{#if readinessStatus}
					<Badge color={readinessBadgeColor(readinessStatus)} class="rounded-none uppercase tracking-[0.14em] text-[9px]">Readiness: {readinessStatus}</Badge>
				{/if}
				{#if patchProposal}
					<Badge color="blue" class="rounded-none uppercase tracking-[0.14em] text-[9px]">Patch Proposed ({patchProposal.operations.length})</Badge>
				{/if}
			</div>

			{#if contextDrift}
				<div class="mb-4 border border-amber-700/60 bg-amber-900/20 px-3 py-2 text-[11px] text-amber-200">
					Document updated during this session. Review generated patches before applying.
				</div>
			{/if}

	    	<PRDChatTranscript
	      	{chatMessages}
	      	{busy}
      	{starting}
      	{assistantDraft}
    	/>

			{#if patchProposal}
				<div class="p-3 bg-blue-900/20 border border-blue-800/60 rounded-none mb-4 flex items-center justify-between gap-3">
					<div class="text-[11px] text-blue-100">
						Patch proposal ready for review. Accept to apply to the document editor.
					</div>
					<div class="flex items-center gap-2">
						<Button size="xs" color="alternative" class="border-gray-600 text-gray-200 px-3 py-1" onclick={rejectPatchProposal}>Reject</Button>
						<Button size="xs" color="alternative" class="bg-[#86BC25] text-black font-bold px-4 py-1" onclick={handleFinalize} disabled={!finalContent}>Accept Patch</Button>
					</div>
				</div>
			{:else if finalContent}
				<div class="p-3 bg-[#86BC25]/10 border border-[#86BC25]/30 rounded-none mb-4 flex justify-between items-center">
					<Badge color="green" class="bg-[#86BC25] text-black font-bold">Document Ready</Badge>
					<Button size="xs" color="alternative" class="bg-[#86BC25] text-black font-bold px-4 py-1" onclick={handleFinalize}>Review & Save</Button>
				</div>
			{/if}

	    	<PRDChatComposer
	      	{chatInput}
	      	disabled={!chatSocket || starting}
				{chatProviderProfiles}
				{chatProviderProfileID}
				{chatThinkingLevel}
	      	onChatInputChange={(value) => chatInput = value}
				onProviderProfileChange={handleComposerProviderChange}
				onThinkingLevelChange={handleComposerThinkingLevelChange}
	      	onSend={sendChatMessage}
	    	/>
		</div>

	</div>
</aside>

<style>
	.prd-chat-drawer {
		width: 100%;
		height: 100%;
		min-width: 0;
		flex-shrink: 0;
	}
</style>
