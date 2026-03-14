<script lang="ts">
  import EmptyState from '$lib/components/EmptyState.svelte';
  import { marked } from 'marked';
  import { Button, Input, Select, Textarea, Toolbar, ToolbarButton, ToolbarGroup } from 'flowbite-svelte';
  import { ArchiveOutline, CloseOutline, CloudArrowUpOutline, EditOutline, RocketOutline, TrashBinOutline } from 'flowbite-svelte-icons';

  interface Props {
    isEditing: boolean;
    selectedDocId: string | null;
    selectedDoc: any;
    editTitle: string;
    editContent: string;
    editProjectID: string;
    projects: any[];
    onEditTitle: (value: string) => void;
    onEditContent: (value: string) => void;
    onEditProjectID: (value: string) => void;
    onStartEdit: () => void;
    onSaveDocument: () => void;
    onCancelEdit: () => void;
    onBuildDoc: () => void;
    onArchiveDoc: () => void;
    onDeleteDoc: () => void;
  }

  let {
    isEditing,
    selectedDocId,
    selectedDoc,
    editTitle,
    editContent,
    editProjectID,
    projects,
    onEditTitle,
    onEditContent,
    onEditProjectID,
    onStartEdit,
    onSaveDocument,
    onCancelEdit,
    onBuildDoc,
    onArchiveDoc,
    onDeleteDoc
  }: Props = $props();

  const renderedContent = $derived.by(() => {
    return marked.parse(editContent || '');
  });

  function appendToContent(fragment: string) {
    onEditContent(editContent + fragment);
  }
</script>

<main class="doc-editor-pane">
  {#if isEditing || selectedDocId || editTitle}
    <div class="editor-frame">
      <div class="editor-top-bar px-8 py-6 border-b border-gray-900">
        {#if isEditing}
          <div class="flex items-center justify-between w-full gap-4">
            <div class="flex items-center gap-4 flex-1">
              <Input
                type="text"
                value={editTitle}
                oninput={(event) => onEditTitle((event.currentTarget as HTMLInputElement).value)}
                placeholder="Document Title"
                class="bg-black border-gray-800 text-white font-bold text-xl flex-1 rounded-none focus:border-[#86BC25] transition-all"
              />
              <Select
                value={editProjectID}
                onchange={(event) => onEditProjectID((event.currentTarget as HTMLSelectElement).value)}
                class="bg-black border-gray-800 text-white w-48 rounded-none"
              >
                {#each projects as p}
                  <option value={p.id}>{p.name}</option>
                {/each}
              </Select>
            </div>
            <div class="flex gap-2">
              <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[9px] px-4 py-1 h-7 rounded-none" onclick={onSaveDocument}>
                <CloudArrowUpOutline size="xs" class="mr-1.5" />
                Save
              </Button>
              <Button color="alternative" class="border-gray-800 text-gray-400 font-bold uppercase text-[9px] px-4 py-1 h-7 rounded-none" onclick={onCancelEdit}>
                <CloseOutline size="xs" class="mr-1.5" />
                Cancel
              </Button>
            </div>
          </div>
        {:else}
          <div class="flex items-center justify-between w-full">
            <div class="flex flex-col">
              <h1 class="title-display uppercase">{editTitle}</h1>
              <div class="mt-1 text-[10px] font-bold text-gray-600 tracking-[0.2em]">{editProjectID}</div>
            </div>
            <div class="flex gap-2">
              <Button color="alternative" class="border-gray-800 text-gray-400 hover:text-white rounded-none font-bold text-[9px] tracking-widest px-3 h-7" onclick={onStartEdit} title="Edit">
                <EditOutline size="xs" class="mr-1.5" />
                EDIT
              </Button>
              <Button color="alternative" class="bg-[#86BC25] text-black font-bold rounded-none text-[9px] tracking-widest px-3 h-7" onclick={onBuildDoc} title="Build">
                <RocketOutline size="xs" class="mr-1.5" />
                BUILD
              </Button>
              <Button color="alternative" class="border-gray-800 text-gray-400 hover:text-white rounded-none px-2 h-7" onclick={onArchiveDoc} title="Archive">
                <ArchiveOutline size="xs" />
              </Button>
              <Button color="red" class="rounded-none border-none px-2 h-7" onclick={onDeleteDoc} title="Delete">
                <TrashBinOutline size="xs" />
              </Button>
            </div>
          </div>
        {/if}
      </div>

      <div class="editor-viewport flex-1 flex">
        {#if isEditing}
          <div class="flex w-full h-full divide-x divide-gray-900 overflow-hidden">
            <div class="flex-1 flex flex-col bg-black">
              <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest">Editor</div>
              <Textarea
                value={editContent}
                oninput={(event) => onEditContent((event.currentTarget as HTMLTextAreaElement).value)}
                rows={20}
                placeholder="Start writing in Markdown..."
                class="bg-black border-none text-gray-300 font-mono text-lg focus:ring-0 resize-none h-full w-full rounded-none px-8 py-6"
              >
                <Toolbar slot="header" embedded class="bg-black border-b border-gray-900 rounded-none px-8">
                  <ToolbarGroup>
                    <ToolbarButton name="Bold" class="text-gray-400 hover:text-white" onclick={() => appendToContent('**bold**')}><span class="font-bold text-xs">B</span></ToolbarButton>
                    <ToolbarButton name="Italic" class="text-gray-400 hover:text-white" onclick={() => appendToContent('_italic_')}><span class="italic text-xs">I</span></ToolbarButton>
                    <ToolbarButton name="List" class="text-gray-400 hover:text-white" onclick={() => appendToContent('\n- ')}><span class="text-xs">L</span></ToolbarButton>
                  </ToolbarGroup>
                </Toolbar>
              </Textarea>
            </div>
            <div class="flex-1 flex flex-col overflow-hidden bg-black">
              <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest">Live Preview</div>
              <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto px-8 py-8">
                {@html renderedContent}
              </div>
            </div>
          </div>
        {:else}
          <div class="flex-1 flex flex-col bg-black">
            <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto px-8 py-8">
              {@html renderedContent}
            </div>
          </div>
        {/if}
      </div>
    </div>
  {:else}
    <EmptyState
      title=""
      description="Select a document from the left to view or edit."
      icon="📄"
    />
  {/if}
</main>

<style>
  .doc-editor-pane {
    background: #000000;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    position: relative;
  }

  .editor-frame {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .title-display {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 800;
    color: #ffffff;
    letter-spacing: -0.02em;
  }

  .editor-viewport {
    flex: 1;
    overflow: hidden;
    display: flex;
  }

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
