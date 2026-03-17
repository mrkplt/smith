<script lang="ts">
	import { sidebarOpen, chatOpen } from '$lib/stores';
	import { hasFeatureCapabilityAccess } from '$lib/feature-capability/access';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
  import { Sidebar, SidebarGroup, SidebarItem, SidebarWrapper, Drawer } from 'flowbite-svelte';
  import { CloseOutline, MessagesOutline } from 'flowbite-svelte-icons';
  import { sineIn } from 'svelte/easing';
  import { shellConfigurationNav, shellRuntimeNav } from '$lib/navigation';
  const canAccessFeatureCapability = $derived(hasFeatureCapabilityAccess());

  let transitionParams = {
    x: -320,
    duration: 200,
    easing: sineIn
  };

  const currentPath = $derived(page.url.pathname);

  function openOperatorChat() {
    if (typeof window !== 'undefined' && window.localStorage.getItem('smith.chat.preferFullScreen') === 'true') {
      const returnTo = encodeURIComponent(page.url.pathname + page.url.search);
      void goto(`/assistant?returnTo=${returnTo}`);
      sidebarOpen.set(false);
      return;
    }
    chatOpen.update(v => !v);
    sidebarOpen.set(false);
  }
</script>

<Drawer 
  bind:open={$sidebarOpen} 
  id="sidebar-drawer" 
  width="default" 
  class="bg-black border-r border-gray-800 p-0 z-50 w-64"
>
  <SidebarWrapper class="bg-black h-full flex flex-col">
    <div class="px-6 py-8 flex items-center justify-between">
      <div class="brand-line flex items-center gap-3">
        <span id="api-dot" class="dot w-3 h-3 rounded-full bg-[#86BC25] shadow-[0_0_10px_rgba(134,188,37,0.8)]" aria-hidden="true"></span>
        <span class="text-2xl font-bold tracking-tighter text-white uppercase font-sans">SMITH</span>
      </div>
      <button 
        class="text-gray-500 hover:text-white transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#86BC25]/70"
        onclick={() => sidebarOpen.set(false)}
        aria-label="Close Sidebar"
      >
        <CloseOutline size="md" />
      </button>
    </div>
    
    <div class="px-0 flex-1">
      <SidebarGroup>
        <div class="px-6 pb-2 text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Runtime</div>
        {#each shellRuntimeNav as item}
          {#if String(item.id) !== 'feature-capability' || canAccessFeatureCapability}
          {@const active = currentPath.startsWith(item.href)}
          <SidebarItem
            href={item.href}
            {active}
            onclick={() => sidebarOpen.set(false)}
            class="group text-gray-400 hover:text-[#86BC25] hover:bg-white/5 rounded-none transition-all py-4 px-6 border-l-2 border-transparent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#86BC25]/70 {active ? 'border-[#86BC25] text-white bg-white/5' : ''}"
          >
            {#snippet icon()}
              <div class="flex items-center gap-3">
                <item.icon size="sm" class="transition duration-75 group-hover:text-[#86BC25] {active ? 'text-[#86BC25]' : ''}" />
                <span class="font-bold uppercase tracking-tight text-sm">{item.label}</span>
              </div>
            {/snippet}
          </SidebarItem>
          {/if}
        {/each}

        <div class="px-6 pt-5 pb-2 text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Configuration</div>
        {#each shellConfigurationNav as item}
          {#if String(item.id) !== 'feature-capability' || canAccessFeatureCapability}
          {@const active = currentPath.startsWith(item.href)}
          <SidebarItem
            href={item.href}
            {active}
            onclick={() => sidebarOpen.set(false)}
            class="group text-gray-400 hover:text-[#86BC25] hover:bg-white/5 rounded-none transition-all py-4 px-6 border-l-2 border-transparent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#86BC25]/70 {active ? 'border-[#86BC25] text-white bg-white/5' : ''}"
          >
            {#snippet icon()}
              <div class="flex items-center gap-3">
                <item.icon size="sm" class="transition duration-75 group-hover:text-[#86BC25] {active ? 'text-[#86BC25]' : ''}" />
                <span class="font-bold uppercase tracking-tight text-sm">{item.label}</span>
              </div>
            {/snippet}
          </SidebarItem>
          {/if}
        {/each}

        <div class="px-6 pt-5 pb-2 text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Preferences</div>
        <SidebarItem
          onclick={openOperatorChat}
          class="group text-gray-400 hover:text-blue-500 hover:bg-white/5 rounded-none transition-all py-4 px-6 border-l-2 border-transparent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/70"
        >
          {#snippet icon()}
            <div class="flex items-center gap-3">
              <MessagesOutline size="sm" class="transition duration-75 group-hover:text-blue-500" />
              <span class="font-bold uppercase tracking-tight text-sm">Operator Chat</span>
            </div>
          {/snippet}
        </SidebarItem>
      </SidebarGroup>
    </div>

    <div class="mt-auto p-6 border-t border-gray-900">
      <div class="text-[10px] font-bold text-gray-600 uppercase tracking-[0.2em]">
        System v1.0.4
      </div>
    </div>
  </SidebarWrapper>
</Drawer>

<style>
  :global(#sidebar-drawer) {
    background-color: #000000 !important;
  }
</style>
