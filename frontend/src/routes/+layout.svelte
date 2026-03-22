<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import ChatPanel from '$lib/components/chat/ChatPanel.svelte';
	import { appState, chatOpen, chatType, themeMode } from '$lib/stores';
	import { buildGlobalChatContext } from '$lib/chat/context';
	import { onMount, onDestroy } from 'svelte';
	import { connectLayoutStreams, initLayoutState } from '$lib/streams/layout-streams';
	import { isChatEnabled } from '$lib/feature-flags';

	let { children } = $props();

	let disconnectStreams: (() => void) | null = null;
	const drawerPath = $derived(page.url.pathname + page.url.search);
	const isFullChatRoute = $derived(page.url.pathname.startsWith('/assistant'));
	const chatEnabled = $derived(isChatEnabled());
	const globalChatContext = $derived(buildGlobalChatContext(page.url.pathname, $chatType, $appState));
	const runtimeGatedPath = $derived(
		page.url.pathname === '/' ||
		page.url.pathname.startsWith('/pods') ||
		page.url.pathname.startsWith('/documents') ||
		page.url.pathname.startsWith('/tasks') ||
		page.url.pathname.startsWith('/pod-view')
	);

	onMount(() => {
		if (typeof window !== 'undefined') {
			const savedTheme = window.localStorage.getItem('smith.theme.mode');
			if (savedTheme === 'light' || savedTheme === 'dark') {
				themeMode.set(savedTheme);
			}
		}
		initLayoutState();
		disconnectStreams = connectLayoutStreams();
	});

	onDestroy(() => {
		disconnectStreams?.();
	});

	$effect(() => {
		if (!chatEnabled && $chatOpen) {
			chatOpen.set(false);
		}
	});

	$effect(() => {
		if (typeof document === 'undefined') {
			return;
		}
		document.documentElement.classList.remove('light', 'dark');
		document.documentElement.classList.add($themeMode);
		if (typeof window !== 'undefined') {
			window.localStorage.setItem('smith.theme.mode', $themeMode);
		}
	});

	$effect(() => {
		if (typeof window === 'undefined') {
			return;
		}
		if (!$appState.onboardingChecked || $appState.onboardingReady) {
			return;
		}
		if (!runtimeGatedPath || page.url.pathname.startsWith('/onboarding')) {
			return;
		}
		void goto('/onboarding', { replaceState: true });
	});
</script>

<Toast />

<div class="shell h-screen bg-black flex overflow-hidden">
	<Sidebar />

	<main class="workspace flex-1 max-w-screen-2xl mx-auto px-2 lg:px-4 overflow-y-auto min-h-0">
		{@render children()}
	</main>

	    {#if chatEnabled && $chatOpen && !isFullChatRoute}
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
    overflow: hidden;
  }

  .workspace {
    min-height: 0;
  }
</style>
