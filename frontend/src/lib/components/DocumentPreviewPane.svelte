<script lang="ts">
  interface Props {
    renderedContent: string;
    showHeader?: boolean;
  }

  let { renderedContent, showHeader = false }: Props = $props();
</script>

<div class="preview-pane-shell">
  {#if showHeader}
    <div class="preview-pane-header">Live Preview</div>
  {/if}
  <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto px-8 py-8">
    {@html renderedContent}
  </div>
</div>

<style>
  .markdown-preview {
    color: #334155;
    line-height: 1.7;
    font-size: 1rem;
  }

  .preview-pane-shell {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--surface-1);
  }

  .preview-pane-header {
    padding: 0.5rem 2rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-2);
    font-size: 0.62rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.16em;
    color: #64748b;
  }

  :global(.markdown-preview h1) { font-size: 1.85rem; border-bottom: 1px solid var(--border-subtle); padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 700; color: #0f172a; }
  :global(.markdown-preview h2) { font-size: 1.4rem; border-bottom: 1px solid var(--border-subtle); padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 700; color: #0f172a; }
  :global(.markdown-preview p) { margin-top: 0; margin-bottom: 16px; }
  :global(.markdown-preview ul) { padding-left: 2em; margin-bottom: 16px; list-style-type: disc; }
  :global(.markdown-preview code) { padding: 0.2em 0.4em; margin: 0; font-size: 85%; background-color: rgba(148, 163, 184, 0.25); border-radius: 6px; font-family: var(--mono); }
  :global(.markdown-preview pre) { padding: 16px; overflow: auto; font-size: 85%; line-height: 1.45; background-color: var(--surface-2); border-radius: 8px; border: 1px solid var(--border-subtle); margin-bottom: 16px; }
  :global(.markdown-preview pre code) { background-color: transparent; padding: 0; font-size: 100%; }

  :global(.dark .preview-pane-header) {
    color: #94a3b8;
  }

  :global(.dark .markdown-preview) {
    color: #d1d7e0;
  }

  :global(.dark .markdown-preview h1),
  :global(.dark .markdown-preview h2) {
    color: #fff;
  }

  :global(.dark .markdown-preview code) {
    background-color: rgba(110, 118, 129, 0.4);
    border-radius: 0;
  }

  :global(.dark .markdown-preview pre) {
    border-radius: 0;
  }
</style>
