<script lang="ts">
    import { goto } from '$app/navigation';
    import { page } from '$app/state';
    import TopBar from '$lib/components/TopBar.svelte';
    import ChatPanel from '$lib/components/chat/ChatPanel.svelte';
    import { buildGlobalChatContext, extractCarriedChatContext, sanitizeReturnToPath } from '$lib/chat/context';
    import { appState, chatType } from '$lib/stores';

    const carriedContext = $derived(extractCarriedChatContext(page.url.searchParams));
    const returnTo = $derived(sanitizeReturnToPath(page.url.searchParams.get('returnTo') || '/projects'));
    const currentPath = $derived(page.url.pathname + page.url.search);

    const pageChatContext = $derived.by(() => {
        if (Object.keys(carriedContext).length > 0) {
            return {
                ...carriedContext,
                sessionType: $chatType
            };
        }
        return buildGlobalChatContext(page.url.pathname, $chatType, $appState);
    });

    function closePageChat() {
        void goto(returnTo);
    }
</script>

<TopBar title="Chat" />

<section class="chat-page px-4 pb-4">
    <div class="chat-page-shell">
        <ChatPanel
            mode="page"
            type={$chatType}
            context={pageChatContext}
            currentPath={currentPath}
            onClose={closePageChat}
        />
    </div>
</section>

<style>
    .chat-page {
        height: calc(100vh - 150px);
    }

    .chat-page-shell {
        height: 100%;
        min-height: 520px;
    }
</style>
