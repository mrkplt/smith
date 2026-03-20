<script lang="ts">
	import { sidebarOpen, chatOpen } from '$lib/stores';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { isChatEnabled } from '$lib/feature-flags';
  import { Button, Navbar, NavBrand } from 'flowbite-svelte';
  import { BarsOutline, MessagesOutline } from 'flowbite-svelte-icons';

	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		controls?: Snippet;
	}

	let { title, controls }: Props = $props();
  const chatEnabled = $derived(isChatEnabled());

  function openOperatorChat() {
    if (!chatEnabled) {
      return;
    }
    if (typeof window !== 'undefined' && window.localStorage.getItem('smith.chat.preferFullScreen') === 'true') {
      const returnTo = encodeURIComponent(page.url.pathname + page.url.search);
      void goto(`/assistant?returnTo=${returnTo}`);
      return;
    }
    chatOpen.update(v => !v);
  }
</script>

<Navbar fluid class="bg-black/90 border-b border-gray-800 px-4 py-2 sticky top-0 z-40 backdrop-blur">
  <NavBrand href="/">
    <div class="flex items-center gap-2">
      <span class="text-xl font-bold tracking-tighter text-white uppercase font-sans">SMITH</span>
    </div>
  </NavBrand>

  <div class="flex items-center gap-2 lg:order-2">
    {#if chatEnabled}
      <Button
        color="alternative"
        class="p-2 text-blue-500 hover:bg-white/5 transition-colors hidden lg:flex items-center gap-2"
        onclick={openOperatorChat}
        aria-label="Toggle Chat"
      >
        <MessagesOutline size="md" />
        <span class="uppercase tracking-widest text-[10px] font-bold">Chat</span>
      </Button>
    {/if}
    <Button
      color="alternative"
      class="p-2 text-[#86BC25] hover:bg-white/5 transition-colors lg:hidden"
      onclick={() => sidebarOpen.update(v => !v)}
      aria-label="Toggle Sidebar"
    >
      <BarsOutline size="md" />
    </Button>
  </div>

</Navbar>

<div class="page-header py-6 flex items-center justify-between px-4">
  <h1 class="text-2xl font-bold text-white tracking-tight uppercase border-l-4 border-[#86BC25] pl-4">{title}</h1>
  <div id="page-actions" class="flex items-center gap-2">
    {@render controls?.()}
  </div>
</div>

<style>
  :global(.navbar-ul) {
    background: transparent !important;
  }
  :global(.navbar-ul li a:hover) {
    background-color: transparent !important;
  }
</style>
