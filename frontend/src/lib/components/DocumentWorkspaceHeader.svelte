<script lang="ts">
  import { Button, Input, Select } from 'flowbite-svelte';
  import { CloseOutline, CloudArrowUpOutline, EditOutline, FileLinesOutline } from 'flowbite-svelte-icons';

  interface Props {
    isEditing: boolean;
    editTitle: string;
    editProjectID: string;
    editFormat: 'markdown' | 'json';
    projects: any[];
    onEditTitle: (value: string) => void;
    onEditProjectID: (value: string) => void;
    onEditFormat: (value: 'markdown' | 'json') => void;
    onStartEdit: () => void;
    onSaveDocument: () => void;
    onCancelEdit: () => void;
  }

  let {
    isEditing,
    editTitle,
    editProjectID,
    editFormat,
    projects,
    onEditTitle,
    onEditProjectID,
    onEditFormat,
    onStartEdit,
    onSaveDocument,
    onCancelEdit
  }: Props = $props();
</script>

<div class="editor-top-bar px-6 py-4">
  {#if isEditing}
    <div class="flex items-center justify-between w-full gap-4">
      <div class="flex items-center gap-4 flex-1">
        <Input
          type="text"
          value={editTitle}
          oninput={(event) => onEditTitle((event.currentTarget as HTMLInputElement).value)}
          placeholder="Document Title"
          class="doc-title-input"
        />
        <Select
          value={editProjectID}
          onchange={(event) => onEditProjectID((event.currentTarget as HTMLSelectElement).value)}
          class="doc-select w-48"
        >
          {#each projects as project}
            <option value={project.id}>{project.name}</option>
          {/each}
        </Select>
        <Select
          value={editFormat}
          onchange={(event) => onEditFormat((event.currentTarget as HTMLSelectElement).value as 'markdown' | 'json')}
          class="doc-select w-32"
        >
          <option value="markdown">Markdown</option>
          <option value="json">JSON</option>
        </Select>
      </div>
      <div class="flex gap-2">
        <Button color="alternative" class="smith-btn smith-btn-primary" onclick={onSaveDocument}>
          <CloudArrowUpOutline size="xs" class="mr-1.5" />
          Save
        </Button>
        <Button color="alternative" class="smith-btn" onclick={onCancelEdit}>
          <CloseOutline size="xs" class="mr-1.5" />
          Cancel
        </Button>
      </div>
    </div>
  {:else}
    <div class="title-shell">
      <div class="flex flex-col">
        <h1 class="title-display uppercase">{editTitle}</h1>
        <div class="title-meta">{editProjectID} • {editFormat}</div>
      </div>

      <Button color="alternative" class="smith-btn smith-btn-icon" style="align-self:flex-start" onclick={onStartEdit} title="Edit document">
        <span class="doc-edit-glyph">
          <FileLinesOutline size="xs" />
          <EditOutline size="xs" class="doc-edit-pencil" />
        </span>
      </Button>
    </div>
  {/if}
</div>

<style>
  .title-display {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 800;
    color: #0f172a;
    letter-spacing: -0.02em;
  }

  .title-meta {
    margin-top: 0.22rem;
    color: #64748b;
    font-size: 0.58rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  .editor-top-bar {
    border-bottom: 1px solid var(--border-subtle);
    background: var(--surface-2);
    box-shadow: var(--elevation-1), var(--inner-highlight);
  }

  .title-shell {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    width: 100%;
    gap: 0.6rem;
  }

  .doc-edit-glyph {
    position: relative;
    width: 0.9rem;
    height: 0.9rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  :global(.doc-edit-pencil) {
    position: absolute;
    right: -0.28rem;
    bottom: -0.22rem;
    transform: scale(0.62);
  }

  :global(.doc-title-input),
  :global(.doc-select) {
    border: 1px solid var(--border-subtle) !important;
    background: var(--surface-1) !important;
    color: #0f172a !important;
    border-radius: 8px !important;
  }

  :global(.doc-title-input) {
    font-size: 1.08rem !important;
    font-weight: 700 !important;
  }

  :global(.dark .title-display) {
    color: #f8fafc;
  }

  :global(.dark .title-meta) {
    color: #94a3b8;
  }

  :global(.dark .doc-title-input),
  :global(.dark .doc-select) {
    background: var(--surface-1) !important;
    color: #e2e8f0 !important;
  }
</style>
