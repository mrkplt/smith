<script lang="ts">
  import { Button, Input, Select } from 'flowbite-svelte';
  import { ArchiveOutline, CloseOutline, CloudArrowUpOutline, EditOutline, FileLinesOutline, RocketOutline, TrashBinOutline } from 'flowbite-svelte-icons';

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
    onBuildDoc: () => void;
    onCreateTask: () => void;
    onArchiveDoc: () => void;
    onDeleteDoc: () => void;
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
    onCancelEdit,
    onBuildDoc,
    onCreateTask,
    onArchiveDoc,
    onDeleteDoc
  }: Props = $props();
</script>

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
          {#each projects as project}
            <option value={project.id}>{project.name}</option>
          {/each}
        </Select>
        <Select
          value={editFormat}
          onchange={(event) => onEditFormat((event.currentTarget as HTMLSelectElement).value as 'markdown' | 'json')}
          class="bg-black border-gray-800 text-white w-32 rounded-none"
        >
          <option value="markdown">Markdown</option>
          <option value="json">JSON</option>
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
        <div class="mt-1 text-[10px] font-bold text-gray-600 tracking-[0.2em]">{editProjectID} • {editFormat}</div>
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
        <Button color="alternative" class="border-gray-800 text-[#86BC25] hover:text-white rounded-none font-bold text-[9px] tracking-widest px-3 h-7" onclick={onCreateTask} title="Create Task Contract">
          <FileLinesOutline size="xs" class="mr-1.5" />
          TASK
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

<style>
  .title-display {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 800;
    color: #ffffff;
    letter-spacing: -0.02em;
  }
</style>
