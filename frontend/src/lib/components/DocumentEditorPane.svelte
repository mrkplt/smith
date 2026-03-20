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

<div class="flex-1 flex flex-col bg-black overflow-hidden">
  <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest flex items-center justify-between gap-4">
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
      class="bg-black border-none text-gray-300 font-mono text-base focus:ring-0 resize-none h-full w-full rounded-none px-8 py-6"
    />
  {:else}
    <div class="line-canvas h-full overflow-y-auto px-8 py-6">
      {#each lines as line, index}
        {#if index === activeLineIndex}
          <div class="line-row active-line-row">
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
    color: #69788d;
    font-size: 0.56rem;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  .line-canvas {
    background: linear-gradient(180deg, rgba(8, 14, 22, 0.42), rgba(8, 8, 8, 0));
  }

  .line-row {
    width: 100%;
    border: 1px solid transparent;
    border-radius: 2px;
    padding: 6px 10px;
    margin-bottom: 4px;
    text-align: left;
  }

  .rendered-line-row {
    background: transparent;
    cursor: text;
  }

  .rendered-line-row:hover {
    border-color: rgba(74, 89, 112, 0.28);
    background: rgba(10, 14, 20, 0.75);
  }

  .active-line-row {
    border-color: rgba(134, 188, 37, 0.78);
    background: rgba(134, 188, 37, 0.06);
  }

  .line-source-input {
    width: 100%;
    border: none;
    outline: none;
    resize: none;
    overflow: hidden;
    background: transparent;
    color: #d4ddf0;
    font-size: 1.02rem;
    line-height: 1.8;
    font-family: var(--mono);
  }

  .line-rendered {
    display: block;
    color: #d1d7e0;
    line-height: 1.8;
    font-size: 1.02rem;
    min-height: 1.8em;
  }

  :global(.markdown-line > p) {
    margin: 0;
  }

  :global(.markdown-line > h1),
  :global(.markdown-line > h2),
  :global(.markdown-line > h3) {
    margin: 0;
    color: #fff;
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
    padding: 0.1em 0.35em;
    background-color: rgba(110, 118, 129, 0.4);
    font-family: var(--mono);
    font-size: 85%;
  }

  :global(.markdown-line > pre) {
    margin: 0;
    border: 1px solid #1f2937;
    background: #0b0e12;
    padding: 10px 12px;
    white-space: pre-wrap;
  }

  :global(.line-empty) {
    opacity: 0.28;
    display: inline-block;
    min-width: 1px;
  }
</style>
