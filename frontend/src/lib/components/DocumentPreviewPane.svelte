<script lang="ts">
  interface Props {
    renderedContent: string;
    showHeader?: boolean;
  }

  let { renderedContent, showHeader = false }: Props = $props();
</script>

<div class="flex-1 flex flex-col overflow-hidden bg-black">
  {#if showHeader}
    <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest">Live Preview</div>
  {/if}
  <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto px-8 py-8">
    {@html renderedContent}
  </div>
</div>

<style>
  .markdown-preview {
    color: #d1d7e0;
    line-height: 1.8;
    font-size: 1.1rem;
  }

  :global(.markdown-preview h1) { font-size: 2rem; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 600; color: #fff; }
  :global(.markdown-preview h2) { font-size: 1.5rem; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 600; color: #fff; }
  :global(.markdown-preview p) { margin-top: 0; margin-bottom: 16px; }
  :global(.markdown-preview ul) { padding-left: 2em; margin-bottom: 16px; list-style-type: disc; }
  :global(.markdown-preview code) { padding: 0.2em 0.4em; margin: 0; font-size: 85%; background-color: rgba(110, 118, 129, 0.4); border-radius: 0px; font-family: var(--mono); }
  :global(.markdown-preview pre) { padding: 16px; overflow: auto; font-size: 85%; line-height: 1.45; background-color: #111; border-radius: 0px; border: 1px solid #222; margin-bottom: 16px; }
  :global(.markdown-preview pre code) { background-color: transparent; padding: 0; font-size: 100%; }
</style>
