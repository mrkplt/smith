<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import ChatPanel from '$lib/components/chat/ChatPanel.svelte';
	import { appState, chatOpen, chatType } from '$lib/stores';
	import { buildGlobalChatContext } from '$lib/chat/context';
	import { onMount, onDestroy } from 'svelte';
	import { connectLayoutStreams, initLayoutState } from '$lib/streams/layout-streams';

	let { children } = $props();

	let disconnectStreams: (() => void) | null = null;
	const drawerPath = $derived(page.url.pathname + page.url.search);
	const isFullChatRoute = $derived(page.url.pathname.startsWith('/assistant'));
	const globalChatContext = $derived(buildGlobalChatContext(page.url.pathname, $chatType, $appState));

	onMount(() => {
    document.documentElement.classList.add('dark');
		initLayoutState();
		disconnectStreams = connectLayoutStreams();
	});

	onDestroy(() => {
		disconnectStreams?.();
	});
</script>

<Toast />

<div class="shell min-h-screen bg-black flex overflow-hidden">
	<Sidebar />

	<main class="workspace flex-1 max-w-screen-2xl mx-auto px-4 lg:px-8 overflow-y-auto">
		{@render children()}
	</main>

	    {#if $chatOpen && !isFullChatRoute}
        <div class="w-96 h-screen flex-shrink-0">
            <ChatPanel
				mode="drawer"
				type={$chatType}
				context={globalChatContext}
				currentPath={drawerPath}
				onClose={() => chatOpen.set(false)}
			/>
        </div>
    {/if}
</div>

<style>
  :global(body) {
    background-color: #000000;
    margin: 0;
    padding: 0;
  }
</style>
