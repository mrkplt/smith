<script lang="ts">
  import { Button } from 'flowbite-svelte';
  import { FileLinesOutline } from 'flowbite-svelte-icons';

  interface Props {
    projectIDs: string[];
    projectsWithDocs: Record<string, any[]>;
    selectedDocId: string | null;
    showAll: boolean;
    onShowAllChange: (value: boolean) => void;
    onSelectDocument: (doc: any) => void;
  }

  let { projectIDs, projectsWithDocs, selectedDocId, showAll, onShowAllChange, onSelectDocument }: Props = $props();
</script>

<aside class="doc-list-sidebar">
  <div class="sidebar-controls">
    <Button
      color="alternative"
      class="archive-toggle rounded-none border-gray-800 text-gray-300 hover:text-white font-bold uppercase text-[9px] tracking-widest h-7 px-3"
      onclick={() => onShowAllChange(!showAll)}
      title="Toggle archived documents"
    >
      {showAll ? 'Hide Archived' : 'Show Archived'}
    </Button>
  </div>

  {#each projectIDs as pid}
    <div class="project-group">
      <div class="project-header">
        {pid}
      </div>
      <div class="project-docs">
        {#each projectsWithDocs[pid] as doc}
          <button
            class="doc-item"
            class:active={selectedDocId === doc.id}
            onclick={() => onSelectDocument(doc)}
          >
            <span class="doc-label">{doc.title || "Untitled"}</span>
          </button>
        {/each}
      </div>
    </div>
  {:else}
    <div class="empty-sidebar flex flex-col items-center justify-center gap-3 py-10 opacity-40">
      <FileLinesOutline size="lg" />
      <span class="uppercase text-[10px] font-bold tracking-widest">No documents found</span>
    </div>
  {/each}
</aside>

<style>
  .doc-list-sidebar {
    background: #000000;
    overflow-y: auto;
    padding: 20px 0;
    display: flex;
    flex-direction: column;
    gap: 24px;
    border-right: 1px solid rgba(255, 255, 255, 0.05);
  }

  .sidebar-controls {
    padding: 0 24px;
  }

  .project-header {
    font-size: 0.6rem;
    text-transform: uppercase;
    letter-spacing: 0.2em;
    color: #5c6b7a;
    font-weight: 800;
    margin-bottom: 12px;
    padding-left: 24px;
  }

  .project-docs {
    display: flex;
    flex-direction: column;
  }

  .doc-item {
    background: transparent;
    border: none;
    border-left: 2px solid transparent;
    display: flex;
    align-items: center;
    padding: 12px 24px;
    color: #90a1b7;
    font-size: 0.8rem;
    font-weight: bold;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    text-align: left;
    cursor: pointer;
    transition: all 0.1s;
  }

  .doc-item:hover {
    background: rgba(255, 255, 255, 0.03);
    color: #fff;
  }

  .doc-item.active {
    background: rgba(134, 188, 37, 0.05);
    color: #86BC25;
    border-left-color: #86BC25;
  }

  .empty-sidebar {
    padding: 40px 20px;
    text-align: center;
    color: #5c6b7a;
    font-size: 0.85rem;
  }
</style>
