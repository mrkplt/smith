<script lang="ts">
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { fetchJSON } from '$lib/api';
    import { loadChatSettings, resolveDefaultModel, resolveProviderType } from '$lib/chat/defaults';
    import { isProviderTypeEnabled } from '$lib/feature-flags';
    import { includeSelectedModel, loadProviderModels } from '$lib/providers/models';
    import { chatSession } from '$lib/chat/store.svelte';
    import { buildChatPageURL, contextSignature } from '$lib/chat/context';
    import MessageList from './MessageList.svelte';
    import Composer from './Composer.svelte';
    import ActionCard from './ActionCard.svelte';
    import { CloseOutline, ExpandOutline } from 'flowbite-svelte-icons';

    interface Props {
        type?: string;
        context?: Record<string, string>;
        mode?: 'drawer' | 'page';
        currentPath?: string;
        onClose?: () => void;
    }

    type ThinkingLevel = 'quick' | 'balanced' | 'deep';

    let {
        type = 'prd-refinement',
        context = {},
        mode = 'drawer',
        currentPath = '/projects',
        onClose
    }: Props = $props();

    let providerProfiles = $state<any[]>([]);
    let providerProfileID = $state('');
    let defaultModel = $state('');
    let modelOptions = $state<string[]>([]);
    let modelOptionsBusy = $state(false);
    let thinkingLevel = $state<ThinkingLevel>('balanced');
    let providerApiKey = $state('');
    let debugOpen = $state(false);
    let appliedContextSig = $state('');

    let isDrawer = $derived(mode === 'drawer');
    let availableModelOptions = $derived(includeSelectedModel(modelOptions, defaultModel));

    function buildContext(): Record<string, string> {
        const nextContext: Record<string, string> = { ...(context || {}) };
        nextContext.sessionType = type;
        const resolvedProvider = resolveProviderType(providerProfiles, providerProfileID, '');
        if (resolvedProvider !== '') {
            nextContext.provider = resolvedProvider;
        }
        if (providerProfileID.trim() !== '') {
            nextContext.providerProfileID = providerProfileID.trim();
        }
        const resolvedModel = resolveModelForSession(resolvedProvider);
        if (resolvedModel !== '') {
            nextContext.model = resolvedModel;
        }
        nextContext.thinkingLevel = thinkingLevel;
        if (providerApiKey.trim() !== '') {
            nextContext.providerApiKey = providerApiKey.trim();
        }
        return nextContext;
    }

    let activeContext = $derived(buildContext());
    let activeContextSig = $derived(contextSignature(activeContext));
    let contextNeedsRefresh = $derived(
        chatSession.sessionId !== null && appliedContextSig !== '' && activeContextSig !== appliedContextSig
    );

    let contextLabel = $derived.by(() => {
        const tags: string[] = [];
        if (activeContext.surface) tags.push(activeContext.surface);
        if (activeContext.loopId) tags.push(`loop:${activeContext.loopId}`);
        if (activeContext.documentId) tags.push(`doc:${activeContext.documentId}`);
        if (activeContext.projectId) tags.push(`project:${activeContext.projectId}`);
        return tags.join(' | ');
    });

    async function startSession() {
        const nextContext = { ...activeContext };
        const ok = await chatSession.createSession(type, nextContext);
        if (ok) {
            appliedContextSig = contextSignature(nextContext);
        }
    }

    async function applySettings() {
        if (typeof window !== 'undefined') {
            const normalizedProfileID = providerProfileID.trim();
            if (normalizedProfileID === '') {
                window.localStorage.removeItem('smith.chat.providerProfileID');
            } else {
                window.localStorage.setItem('smith.chat.providerProfileID', normalizedProfileID);
            }
            const resolvedProvider = resolveProviderType(providerProfiles, normalizedProfileID, '');
            if (resolvedProvider === '') {
                window.localStorage.removeItem('smith.chat.provider');
            } else {
                window.localStorage.setItem('smith.chat.provider', resolvedProvider);
            }
            if (defaultModel.trim() === '') {
                window.localStorage.removeItem('smith.chat.defaultModel');
            } else {
                window.localStorage.setItem('smith.chat.defaultModel', defaultModel.trim());
            }
            window.localStorage.setItem('smith.chat.thinkingLevel', thinkingLevel);
        }
        await startSession();
    }

    async function expandToFullScreen() {
        if (!isDrawer) {
            return;
        }
        const url = buildChatPageURL(activeContext, currentPath);
        onClose?.();
        await goto(url);
    }

    onMount(() => {
        void loadProviderProfiles().then(() => {
            if (typeof window !== 'undefined') {
                const settings = loadChatSettings(window.localStorage, providerProfiles);
                providerProfileID = settings.providerProfileID;
                defaultModel = settings.defaultModel;
                thinkingLevel = settings.thinkingLevel;
                providerApiKey = settings.providerApiKey;
                void loadModelOptions(settings.providerProfileID);
            }

            if (chatSession.sessionId) {
                appliedContextSig = contextSignature(buildContext());
                return;
            }

            void startSession();
        });
    });

    async function loadProviderProfiles() {
        try {
            const profiles = await fetchJSON('/v1/providers');
            providerProfiles = Array.isArray(profiles)
                ? profiles.filter((profile) => {
                    const providerTypeEnabled = isProviderTypeEnabled(String(profile?.provider_type || profile?.id || ''));
                    const isAddedProfile = String(profile?.secret_ref || '').trim() !== '';
                    return providerTypeEnabled && isAddedProfile;
                })
                : [];
        } catch {
            providerProfiles = [];
        }
    }

    async function loadModelOptions(nextProfileID: string) {
        const normalizedProfileID = String(nextProfileID || '').trim();
        const profile = providerProfiles.find((item) => String(item?.id || '').trim() === normalizedProfileID);
        const providerTypeHint = String(profile?.provider_type || '').trim();
        const hasCredentialSecret = String(profile?.secret_ref || '').trim() !== '';
        modelOptionsBusy = true;
        try {
            modelOptions = await loadProviderModels(normalizedProfileID, providerTypeHint, hasCredentialSecret);
        } finally {
            modelOptionsBusy = false;
        }
    }

    function selectProviderProfile(nextProfileID: string) {
        providerProfileID = nextProfileID;
        defaultModel = resolveDefaultModel(providerProfiles, nextProfileID, '');
        void loadModelOptions(nextProfileID);
    }

    function resolveModelForSession(resolvedProvider: string): string {
        const selected = resolveDefaultModel(providerProfiles, providerProfileID, defaultModel);
        if (selected !== '') {
            return selected;
        }
        return modelForThinking(resolvedProvider, thinkingLevel);
    }

    function modelForThinking(activeProvider: string, level: ThinkingLevel): string {
        const normalizedProvider = activeProvider.trim().toLowerCase();
        if (normalizedProvider !== '' && normalizedProvider !== 'openai') {
            return '';
        }
        return 'gpt-5.4';
    }
</script>

<div class={`flex flex-col h-full bg-gray-950 shadow-2xl ${isDrawer ? 'border-l border-gray-800' : 'border border-gray-800'}`}>
    <div class="p-4 border-b border-gray-800 flex justify-between items-center bg-gray-900">
        <div>
            <h3 class="text-sm font-bold text-gray-100 uppercase tracking-wider">Smith Chat</h3>
            <div class="text-[10px] text-gray-500 font-mono">{type}</div>
        </div>
        <div class="flex items-center gap-2">
            {#if isDrawer}
                <button
                    class="p-1.5 hover:bg-gray-800 rounded transition-colors text-gray-400 hover:text-gray-100"
                    onclick={expandToFullScreen}
                    aria-label="Expand chat"
                    title="Open full-screen chat"
                >
                    <ExpandOutline class="w-4 h-4" />
                </button>
            {/if}
            <button
                class="px-2 py-1 text-[10px] rounded border border-gray-700 text-gray-300 hover:bg-gray-800"
                onclick={() => debugOpen = !debugOpen}
            >
                {debugOpen ? 'Hide Debug' : 'Show Debug'}
            </button>
            {#if onClose}
                <button onclick={onClose} class="p-1 hover:bg-gray-800 rounded-full transition-colors text-gray-400 hover:text-gray-100">
                    <CloseOutline class="w-5 h-5" />
                </button>
            {/if}
        </div>
    </div>

    {#if debugOpen}
        <div class="p-2 border-b border-gray-800 bg-black text-[10px] text-gray-300 space-y-1">
            <div class="font-mono">session={chatSession.sessionId || 'none'} creating={String(chatSession.isCreating)} streaming={String(chatSession.isStreaming)} messages={chatSession.messages.length}</div>
            <div class="max-h-28 overflow-y-auto space-y-1 border border-gray-900 bg-gray-950 p-1">
                {#each chatSession.debugEvents as event}
                    <div class="font-mono text-gray-400">
                        [{event.at}] {event.type}: {event.data}
                    </div>
                {:else}
                    <div class="text-gray-500">No debug events yet</div>
                {/each}
            </div>
        </div>
    {/if}

    <div class="p-3 border-b border-gray-800 bg-gray-950 space-y-2">
        {#if contextLabel !== ''}
            <div class="text-[10px] text-gray-500 font-mono">Context: {contextLabel}</div>
        {/if}
        {#if contextNeedsRefresh}
            <div class="text-[10px] text-amber-300 bg-amber-900/20 border border-amber-800 px-2 py-1 rounded">
                Page context changed. Start a new session to apply updated context.
            </div>
        {/if}
        <div class="grid grid-cols-1 gap-2">
            <select
                class="bg-gray-900 border border-gray-800 text-gray-100 text-xs rounded px-2 py-2"
                value={providerProfileID}
                oninput={(event) => selectProviderProfile((event.currentTarget as HTMLSelectElement).value)}
            >
                {#if providerProfiles.length === 0}
                    <option value="">Provider Profile: service default</option>
                {:else}
                    {#each providerProfiles as profile}
                        <option value={profile.id}>Provider Profile: {profile.name || profile.id}</option>
                    {/each}
                {/if}
            </select>
            <select
                class="bg-gray-900 border border-gray-800 text-gray-100 text-xs rounded px-2 py-2"
                value={defaultModel}
                oninput={(event) => defaultModel = (event.currentTarget as HTMLSelectElement).value}
            >
                <option value="">Model: profile default</option>
                {#if modelOptionsBusy}
                    <option value={defaultModel} disabled>{defaultModel !== '' ? `Model: ${defaultModel}` : 'Loading models...'}</option>
                {/if}
                {#each availableModelOptions as model}
                    <option value={model}>Model: {model}</option>
                {/each}
            </select>
            <div class="flex gap-2">
                <select
                    class="flex-1 bg-gray-900 border border-gray-800 text-gray-100 text-xs rounded px-2 py-2"
                    value={thinkingLevel}
                    oninput={(event) => thinkingLevel = (event.currentTarget as HTMLSelectElement).value as ThinkingLevel}
                >
                    <option value="quick">Thinking: quick</option>
                    <option value="balanced">Thinking: balanced</option>
                    <option value="deep">Thinking: deep</option>
                </select>
                <button
                    class="px-3 py-2 text-xs font-semibold rounded border border-gray-700 text-gray-200 hover:bg-gray-800"
                    onclick={applySettings}
                    disabled={chatSession.isCreating || chatSession.isStreaming}
                >
                    New Session
                </button>
            </div>
        </div>
    </div>

    {#if chatSession.error}
        <div class="p-2 bg-red-900/20 border-b border-red-900/50 text-red-400 text-xs text-center">
            {chatSession.error}
        </div>
    {/if}

    {#if chatSession.isCreating}
        <div class="p-2 bg-blue-900/20 border-b border-blue-900/50 text-blue-300 text-xs text-center">
            Starting chat session...
        </div>
    {/if}

    <MessageList messages={chatSession.messages} />
    
    {#if chatSession.structuredResults.length > 0}
        <div class="px-4 pb-4 space-y-3">
            {#each chatSession.structuredResults as result}
                <ActionCard {result} />
            {/each}
        </div>
    {/if}

    <Composer disabled={chatSession.isCreating || !chatSession.sessionId} />
</div>
