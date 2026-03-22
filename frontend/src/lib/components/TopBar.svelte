<script lang="ts">
	import { sidebarOpen, chatOpen } from '$lib/stores';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { isChatEnabled } from '$lib/feature-flags';
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

<header class="topbar sticky top-0 z-40 px-4 py-2">
  <div class="topbar-row">
    <div class="topbar-left">
      <a class="brand" href="/">SMITH</a>
      <h1 class="inline-title">{title}</h1>
    </div>

    <div class="topbar-right">
      <div id="page-actions" class="page-actions">
      {@render controls?.()}
      </div>
      {#if chatEnabled}
      <button
        class="icon-btn chat-btn hidden lg:flex"
        onclick={openOperatorChat}
        aria-label="Toggle Chat"
      >
        <MessagesOutline size="md" />
        <span class="chat-label">Chat</span>
      </button>
      {/if}
      <button
        class="icon-btn menu-btn flex lg:hidden"
        onclick={() => sidebarOpen.update(v => !v)}
        aria-label="Toggle Sidebar"
      >
        <BarsOutline size="md" />
      </button>
    </div>
  </div>
</header>

<style>
  .topbar {
    border-bottom: 1px solid var(--border-subtle, rgba(31, 41, 55, 0.85));
    background: var(--surface-2, rgba(2, 6, 23, 0.9));
    backdrop-filter: blur(8px);
    box-shadow: var(--elevation-1);
    border-radius: 10px;
    overflow: visible;
    margin-inline: 1rem;
  }

  .topbar-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 46px;
    gap: 1rem;
  }

  .topbar-left {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    min-width: 0;
  }

  .brand {
    color: var(--shell-topbar-text, #ffffff);
    font-size: 1.65rem;
    font-weight: 800;
    letter-spacing: -0.03em;
    text-transform: uppercase;
    text-decoration: none;
    line-height: 1;
  }

  .inline-title {
    color: var(--shell-topbar-text, #ffffff);
    border-left: 4px solid var(--accent, #86bc25);
    padding-left: 0.7rem;
    margin: 0;
    line-height: 1;
    text-transform: uppercase;
    font-weight: 800;
    font-size: 1.65rem;
    letter-spacing: -0.02em;
    max-width: min(44vw, 540px);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .topbar-right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .page-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .icon-btn {
    border: 1px solid var(--shell-topbar-button-border, rgba(148, 163, 184, 0.28));
    background: var(--shell-topbar-button-bg, rgba(2, 6, 23, 0.45));
    border-radius: 0.45rem;
    color: var(--accent, #86bc25);
    padding: 0.38rem 0.5rem;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    cursor: pointer;
  }

  .icon-btn:hover {
    background: var(--shell-topbar-button-bg-hover, rgba(15, 23, 42, 0.65));
  }

  .chat-btn {
    color: var(--shell-chat-button, #60a5fa);
  }

  .chat-label {
    font-size: 0.62rem;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    font-weight: 700;
  }

  .menu-btn {
    color: var(--accent, #86bc25);
  }

  @media (max-width: 900px) {
    #page-actions {
      display: none;
    }

    .inline-title {
      max-width: min(58vw, 290px);
      font-size: 1.45rem;
      padding-left: 0.55rem;
      border-left-width: 3px;
    }

    .brand {
      font-size: 1.45rem;
    }
  }

  @media (max-width: 640px) {
    .inline-title {
      font-size: 1.2rem;
      max-width: min(52vw, 180px);
    }

    .brand {
      font-size: 1.2rem;
    }
  }
</style>
