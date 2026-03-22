<script lang="ts">
  import { Textarea } from 'flowbite-svelte';
  import { marked } from 'marked';

  interface Props {
    editContent: string;
    format: 'markdown' | 'json';
    onEditContent: (value: string) => void;
    onFocusContextChange?: (focus: {
      lineIndex: number | null;
      sectionId: string;
      selectionText: string;
    }) => void;
  }

  let { editContent, format, onEditContent, onFocusContextChange }: Props = $props();

  let activeLineIndex = $state<number | null>(null);
  let activeLineEditor = $state<HTMLTextAreaElement | null>(null);
  let lastFocusedLine: number | null = null;
  let preferredColumn: number | null = null;

  const lines = $derived.by(() => {
    const normalized = editContent.replaceAll('\r\n', '\n').replaceAll('\r', '\n');
    return normalized.split('\n');
  });

  function setActiveLine(index: number) {
    if (format !== 'markdown') {
      return;
    }
    activeLineIndex = index;
  }

  function clearActiveLine() {
    activeLineIndex = null;
    preferredColumn = null;
  }

  function moveActiveLine(delta: number, column: number | null = null) {
    if (activeLineIndex === null) {
      return;
    }
    const nextIndex = Math.max(0, Math.min(lines.length - 1, activeLineIndex + delta));
    if (nextIndex === activeLineIndex) {
      return;
    }
    preferredColumn = column;
    activeLineIndex = nextIndex;
  }

  function replaceLines(nextLines: string[]) {
    onEditContent(nextLines.join('\n'));
  }

  function updateActiveLine(value: string) {
    if (activeLineIndex === null) {
      return;
    }

    const normalized = value.replaceAll('\r\n', '\n').replaceAll('\r', '\n');
    const replacement = normalized.split('\n');
    const nextLines = [...lines];
    nextLines.splice(activeLineIndex, 1, ...replacement);
    replaceLines(nextLines);
    activeLineIndex = activeLineIndex + replacement.length - 1;
    preferredColumn = replacement[replacement.length - 1].length;
  }

  function renderMarkdownLine(line: string): string {
    if (line.trim() === '') {
      return '<span class="line-empty">&nbsp;</span>';
    }
    return marked.parse(line) as string;
  }

  function slugifyHeading(value: string): string {
    const normalized = value
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '_')
      .replace(/^_+|_+$/g, '');
    return normalized;
  }

  function sectionIDForLine(lineIndex: number | null): string {
    if (lineIndex === null || lineIndex < 0 || lineIndex >= lines.length) {
      return '';
    }
    for (let index = lineIndex; index >= 0; index -= 1) {
      const line = lines[index] || '';
      const match = line.match(/^#{1,6}\s+(.+)$/);
      if (!match) {
        continue;
      }
      const section = slugifyHeading(match[1] || '');
      if (section !== '') {
        return section;
      }
    }
    return '';
  }

  function handleLineKeydown(event: KeyboardEvent) {
    if (activeLineIndex === null) {
      return;
    }

    const editor = event.currentTarget as HTMLTextAreaElement;
    const cursor = editor.selectionStart ?? 0;
    const currentLine = lines[activeLineIndex] ?? '';

    if (event.key === 'Escape') {
      event.preventDefault();
      clearActiveLine();
      return;
    }

    if (!event.shiftKey && !event.metaKey && !event.ctrlKey && !event.altKey && event.key === 'ArrowUp') {
      event.preventDefault();
      moveActiveLine(-1, cursor);
      return;
    }

    if (!event.shiftKey && !event.metaKey && !event.ctrlKey && !event.altKey && event.key === 'ArrowDown') {
      event.preventDefault();
      moveActiveLine(1, cursor);
      return;
    }

    if (!event.shiftKey && !event.metaKey && !event.ctrlKey && !event.altKey && event.key === 'Enter') {
      event.preventDefault();
      const before = currentLine.slice(0, cursor);
      const after = currentLine.slice(cursor);
      const nextLines = [...lines];
      nextLines.splice(activeLineIndex, 1, before, after);
      replaceLines(nextLines);
      activeLineIndex = activeLineIndex + 1;
      preferredColumn = 0;
      return;
    }

    if (!event.shiftKey && !event.metaKey && !event.ctrlKey && !event.altKey && event.key === 'Backspace' && cursor === 0 && activeLineIndex > 0) {
      event.preventDefault();
      const previous = lines[activeLineIndex - 1] ?? '';
      const nextLines = [...lines];
      nextLines.splice(activeLineIndex - 1, 2, previous + currentLine);
      replaceLines(nextLines);
      activeLineIndex = activeLineIndex - 1;
      preferredColumn = previous.length;
      return;
    }

    if (!event.shiftKey && !event.metaKey && !event.ctrlKey && !event.altKey && event.key === 'Delete' && cursor === currentLine.length && activeLineIndex < lines.length - 1) {
      event.preventDefault();
      const next = lines[activeLineIndex + 1] ?? '';
      const nextLines = [...lines];
      nextLines.splice(activeLineIndex, 2, currentLine + next);
      replaceLines(nextLines);
      preferredColumn = currentLine.length;
    }
  }

  function handleActiveBlur() {
    setTimeout(() => {
      if (document.activeElement !== activeLineEditor) {
        clearActiveLine();
      }
    }, 0);
  }

  $effect(() => {
    if (format === 'json') {
      clearActiveLine();
    }
  });

  $effect(() => {
    if (activeLineIndex === null) {
      return;
    }
    if (activeLineIndex > lines.length - 1) {
      activeLineIndex = lines.length - 1;
    }
  });

  $effect(() => {
    if (activeLineIndex === null || activeLineEditor === null) {
      if (activeLineIndex === null) {
        lastFocusedLine = null;
      }
      return;
    }
    if (lastFocusedLine === activeLineIndex && preferredColumn === null) {
      return;
    }
    activeLineEditor.focus();
    const nextColumn = preferredColumn ?? activeLineEditor.value.length;
    const clamped = Math.max(0, Math.min(nextColumn, activeLineEditor.value.length));
    activeLineEditor.setSelectionRange(clamped, clamped);
    preferredColumn = null;
    lastFocusedLine = activeLineIndex;
  });

  $effect(() => {
    if (!onFocusContextChange || format !== 'markdown') {
      return;
    }
    if (activeLineIndex === null) {
      onFocusContextChange({
        lineIndex: null,
        sectionId: '',
        selectionText: ''
      });
      return;
    }

    const line = (lines[activeLineIndex] || '').trim();
    onFocusContextChange({
      lineIndex: activeLineIndex,
      sectionId: sectionIDForLine(activeLineIndex),
      selectionText: line
    });
  });
</script>

<div class="editor-pane-shell">
  <div class="editor-pane-header">
    <span>Editor</span>
    {#if format === 'markdown'}
      <span class="line-status">Live markdown rendering</span>
    {:else}
      <span class="line-status">JSON source editing</span>
    {/if}
  </div>

  {#if format === 'json'}
    <Textarea
      value={editContent}
      oninput={(event) => onEditContent((event.currentTarget as HTMLTextAreaElement).value)}
      rows={20}
      placeholder="Paste canonical PRD JSON..."
      class="json-editor-textarea"
    />
  {:else}
    <div class="line-canvas h-full overflow-y-auto px-8 py-6">
      {#each lines as line, index}
        {#if index === activeLineIndex}
          <div class="line-row active-line-row" class:empty-active-line={line.trim() === ''}>
            <textarea
              bind:this={activeLineEditor}
              class="line-source-input"
              value={line}
              rows={1}
              oninput={(event) => updateActiveLine((event.currentTarget as HTMLTextAreaElement).value)}
              onkeydown={handleLineKeydown}
              onblur={handleActiveBlur}
            ></textarea>
          </div>
        {:else}
          <button class="line-row rendered-line-row" onclick={() => setActiveLine(index)}>
            <span class="line-rendered markdown-line">{@html renderMarkdownLine(line)}</span>
          </button>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .line-status {
    color: #64748b;
    font-size: 0.56rem;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  .line-canvas {
    background: var(--surface-1);
  }

  .editor-pane-shell {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--surface-1);
  }

  .editor-pane-header {
    padding: 0.5rem 2rem;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-2);
    font-size: 0.62rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.16em;
    color: #64748b;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  :global(.json-editor-textarea) {
    background: var(--surface-1) !important;
    border: none !important;
    color: #334155 !important;
    font-family: var(--mono) !important;
    font-size: 0.94rem !important;
    line-height: 1.6 !important;
    resize: none !important;
    height: 100% !important;
    width: 100% !important;
    border-radius: 0 !important;
    padding: 1.5rem 2rem !important;
  }

  .line-row {
    width: 100%;
    border: 1px solid transparent;
    border-radius: 0;
    padding: 3px 8px;
    margin-bottom: 1px;
    text-align: left;
  }

  .rendered-line-row {
    background: transparent;
    cursor: text;
  }

  .rendered-line-row:hover {
    border-color: transparent;
    background: rgba(148, 163, 184, 0.14);
  }

  .rendered-line-row:focus-visible {
    outline: none;
    border-color: transparent;
  }

  .active-line-row {
    border-color: transparent;
    background: transparent;
    border-left: 2px solid rgba(134, 188, 37, 0.65);
    padding-left: 6px;
  }

  .active-line-row.empty-active-line {
    border-left-color: transparent;
    padding-left: 8px;
  }

  .line-source-input {
    width: 100%;
    border: none;
    outline: none;
    resize: none;
    overflow: hidden;
    background: transparent;
    color: #1e293b;
    font-size: 0.96rem;
    line-height: 1.65;
    font-family: inherit;
  }

  .line-rendered {
    display: block;
    color: #334155;
    line-height: 1.65;
    font-size: 0.96rem;
    min-height: 1.65em;
  }

  :global(.markdown-line > p) {
    margin: 0;
  }

  :global(.markdown-line > h1),
  :global(.markdown-line > h2),
  :global(.markdown-line > h3) {
    margin: 0;
    color: #0f172a;
    font-weight: 650;
  }

  :global(.markdown-line > h1) { font-size: 2rem; }
  :global(.markdown-line > h2) { font-size: 1.5rem; }
  :global(.markdown-line > h3) { font-size: 1.25rem; }

  :global(.markdown-line > ul),
  :global(.markdown-line > ol) {
    margin: 0;
    padding-left: 1.4rem;
  }

  :global(.markdown-line code) {
    padding: 0;
    background: transparent;
    border-bottom: 1px solid rgba(100, 116, 139, 0.35);
    font-family: var(--mono);
    font-size: 90%;
  }

  :global(.markdown-line > pre) {
    margin: 0;
    border: 1px solid var(--border-subtle);
    background: var(--surface-2);
    padding: 10px 12px;
    white-space: pre-wrap;
  }

  :global(.line-empty) {
    opacity: 0.28;
    display: inline-block;
    min-width: 1px;
  }

  :global(.dark .editor-pane-header),
  :global(.dark .line-status) {
    color: #94a3b8;
  }

  :global(.dark .json-editor-textarea),
  :global(.dark .line-source-input),
  :global(.dark .line-rendered) {
    color: #d1d5db !important;
  }

  :global(.dark .rendered-line-row:hover) {
    background: rgba(15, 23, 42, 0.42);
  }

  :global(.dark .markdown-line > h1),
  :global(.dark .markdown-line > h2),
  :global(.dark .markdown-line > h3) {
    color: #f8fafc;
  }
</style>
