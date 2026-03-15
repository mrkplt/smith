<script lang="ts">
	import '../app.css';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import ChatPanel from '$lib/components/chat/ChatPanel.svelte';
	import { chatOpen, chatType } from '$lib/stores';
	import { onMount, onDestroy } from 'svelte';
	import { connectLayoutStreams, initLayoutState } from '$lib/streams/layout-streams';

	let { children } = $props();

	let disconnectStreams: (() => void) | null = null;

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

    {#if $chatOpen}
        <div class="w-96 h-screen flex-shrink-0">
            <ChatPanel type={$chatType} onClose={() => chatOpen.set(false)} />
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
