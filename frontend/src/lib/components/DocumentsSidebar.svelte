<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ChevronRightOutline, FileLinesOutline } from 'flowbite-svelte-icons';

  interface Props {
    projectIDs: string[];
    projectsWithDocs: Record<string, any[]>;
    selectedDocId: string | null;
    showAll: boolean;
    searchQuery: string;
    tasksEnabled: boolean;
    onShowAllChange: (value: boolean) => void;
    onSearchQueryChange: (value: string) => void;
    onSelectDocument: (doc: any) => void;
    onBuildDocument: (doc: any) => void;
    onCreateTaskFromDocument: (doc: any) => void;
    onArchiveDocument: (doc: any) => void;
    onDeleteDocument: (doc: any) => void;
  }

  let {
    projectIDs,
    projectsWithDocs,
    selectedDocId,
    showAll,
    searchQuery,
    tasksEnabled,
    onShowAllChange,
    onSearchQueryChange,
    onSelectDocument,
    onBuildDocument,
    onCreateTaskFromDocument,
    onArchiveDocument,
    onDeleteDocument
  }: Props = $props();

  let collapsedProjects = $state<Record<string, boolean>>({});
  let collapsedLineages = $state<Record<string, boolean>>({});
  let optionsOpen = $state(false);
  let optionsWrapEl: HTMLDivElement | null = null;
  let activeDocMenuID = $state<string | null>(null);

  const projectTrees = $derived.by(() => {
    const out: Array<{ projectID: string; lineages: Array<{ key: string; root: any; children: any[] }> }> = [];
    for (const projectID of projectIDs) {
      const docs = projectsWithDocs[projectID] || [];
      const grouped = new Map<string, any[]>();
      for (const doc of docs) {
        const key = lineageKey(doc);
        if (!grouped.has(key)) {
          grouped.set(key, []);
        }
        grouped.get(key)!.push(doc);
      }

      const lineages: Array<{ key: string; root: any; children: any[] }> = [];
      for (const [key, docsInLineage] of grouped.entries()) {
        const root = pickLineageRoot(key, docsInLineage);
        const children = docsInLineage
          .filter((doc) => String(doc.id || '') !== String(root.id || ''))
          .sort((a, b) => artifactOrder(a) - artifactOrder(b));
        lineages.push({ key, root, children });
      }
      lineages.sort((a, b) => displayTitle(a.root).localeCompare(displayTitle(b.root)));
      out.push({ projectID, lineages });
    }
    return out;
  });

  $effect(() => {
    let projectUpdated = false;
    const nextProjects = { ...collapsedProjects };
    for (const tree of projectTrees) {
      if (!(tree.projectID in nextProjects)) {
        nextProjects[tree.projectID] = true;
        projectUpdated = true;
      }
    }
    if (projectUpdated) {
      collapsedProjects = nextProjects;
    }

    let lineageUpdated = false;
    const nextLineages = { ...collapsedLineages };
    for (const tree of projectTrees) {
      for (const lineage of tree.lineages) {
        if (!(lineage.key in nextLineages)) {
          nextLineages[lineage.key] = true;
          lineageUpdated = true;
        }
      }
    }
    if (lineageUpdated) {
      collapsedLineages = nextLineages;
    }
  });

  function toggleProject(projectID: string) {
    collapsedProjects = { ...collapsedProjects, [projectID]: !collapsedProjects[projectID] };
  }

  function toggleLineage(key: string) {
    collapsedLineages = { ...collapsedLineages, [key]: !collapsedLineages[key] };
  }

  function expandAll() {
    const projectState: Record<string, boolean> = {};
    const lineageState: Record<string, boolean> = {};
    for (const tree of projectTrees) {
      projectState[tree.projectID] = false;
      for (const lineage of tree.lineages) {
        lineageState[lineage.key] = false;
      }
    }
    collapsedProjects = projectState;
    collapsedLineages = lineageState;
    optionsOpen = false;
  }

  function collapseAll() {
    const projectState: Record<string, boolean> = {};
    const lineageState: Record<string, boolean> = {};
    for (const tree of projectTrees) {
      projectState[tree.projectID] = true;
      for (const lineage of tree.lineages) {
        lineageState[lineage.key] = true;
      }
    }
    collapsedProjects = projectState;
    collapsedLineages = lineageState;
    optionsOpen = false;
  }

  function toggleShowArchived() {
    onShowAllChange(!showAll);
    optionsOpen = false;
  }

  function handleWindowPointerDown(event: MouseEvent) {
    if (!optionsOpen || !optionsWrapEl) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Node)) {
      return;
    }
    if (!optionsWrapEl.contains(target)) {
      optionsOpen = false;
    }

    if (activeDocMenuID) {
      const host = target instanceof Element ? target.closest(`[data-doc-menu-host="${activeDocMenuID}"]`) : null;
      if (!host) {
        activeDocMenuID = null;
      }
    }
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      optionsOpen = false;
      activeDocMenuID = null;
    }
  }

  function selectDocumentFromSidebar(doc: any) {
    activeDocMenuID = null;
    onSelectDocument(doc);
  }

  function toggleDocMenu(docID: string) {
    activeDocMenuID = activeDocMenuID === docID ? null : docID;
  }

  function runDocAction(action: 'build' | 'task' | 'archive' | 'delete', doc: any) {
    activeDocMenuID = null;
    if (action === 'build') {
      onBuildDocument(doc);
      return;
    }
    if (action === 'task') {
      onCreateTaskFromDocument(doc);
      return;
    }
    if (action === 'archive') {
      onArchiveDocument(doc);
      return;
    }
    onDeleteDocument(doc);
  }

  onDestroy(() => {
    optionsOpen = false;
  });

  function isGenerated(doc: any): boolean {
    const id = String(doc?.id || '');
    if (id.startsWith('prd-') || id.startsWith('tech-spec-') || id.startsWith('implementation-plan-')) {
      return true;
    }
    return String(doc?.metadata?.loop_id || '').trim() !== '';
  }

  function loopIDFor(doc: any): string {
    const metadataLoop = String(doc?.metadata?.loop_id || '').trim();
    if (metadataLoop) {
      return metadataLoop;
    }
    const id = String(doc?.id || '');
    const prefixed = id.match(/^(?:prd|tech-spec|implementation-plan)-(.+)$/);
    if (prefixed && prefixed[1]) {
      return prefixed[1];
    }
    return '';
  }

  function lineageKey(doc: any): string {
    const metadataRoot = String(doc?.metadata?.document_lineage_root_id || '').trim();
    if (metadataRoot) {
      return `root:${metadataRoot}`;
    }
    const metadataDocRoot = String(doc?.metadata?.document_id || '').trim();
    if (metadataDocRoot) {
      return `root:${metadataDocRoot}`;
    }
    const loopID = loopIDFor(doc);
    if (loopID) {
      return `loop:${loopID}`;
    }
    return `doc:${String(doc?.id || '')}`;
  }

  function pickLineageRoot(key: string, docs: any[]): any {
    if (key.startsWith('root:')) {
      const rootID = key.replace('root:', '');
      const root = docs.find((doc) => String(doc?.id || '') === rootID);
      if (root) {
        return root;
      }
    }
    const nonGenerated = docs.find((doc) => !isGenerated(doc));
    if (nonGenerated) {
      return nonGenerated;
    }
    return docs[0];
  }

  function artifactOrder(doc: any): number {
    const id = String(doc?.id || '');
    if (id.startsWith('prd-')) return 0;
    if (id.startsWith('tech-spec-')) return 1;
    if (id.startsWith('implementation-plan-')) return 2;
    return 3;
  }

  function displayTitle(doc: any): string {
    const raw = String(doc?.title || '').trim();
    if (raw && raw.toLowerCase() !== 'untitled' && raw.toLowerCase() !== 'untitled document') {
      return raw;
    }
    const id = String(doc?.id || '');
    if (id.startsWith('prd-')) return 'Generated PRD';
    if (id.startsWith('tech-spec-')) return 'Generated Tech Spec';
    if (id.startsWith('implementation-plan-')) return 'Generated Implementation Plan';
    if (isGenerated(doc)) return 'Generated Document';
    return 'Untitled';
  }

  function docMetaSuffix(doc: any): string {
    const loopID = loopIDFor(doc);
    if (!loopID) {
      return '';
    }
    return `(${loopID.slice(0, 12)})`;
  }

  function displayProjectLabel(projectID: string): string {
    const normalized = String(projectID || '').trim();
    return normalized === '' ? 'Archived' : normalized;
  }
</script>

<svelte:window onmousedown={handleWindowPointerDown} onkeydown={handleWindowKeydown} />

<aside class="doc-list-sidebar">
  <div class="sidebar-controls">
    <input
      class="sidebar-search smith-filter-control"
      type="search"
      placeholder="Search documents..."
      value={searchQuery}
      oninput={(event) => onSearchQueryChange((event.currentTarget as HTMLInputElement).value)}
      aria-label="Search documents"
    />

    <div class="options-wrap" bind:this={optionsWrapEl}>
      <button class="options-trigger" onclick={() => (optionsOpen = !optionsOpen)} aria-label="Document tree options">...</button>
      {#if optionsOpen}
        <div class="options-menu">
          <button class="options-item" onclick={toggleShowArchived}>{showAll ? 'Hide archived' : 'Show archived'}</button>
          <div class="options-divider"></div>
          <button class="options-item" onclick={expandAll}>Expand all</button>
          <button class="options-item" onclick={collapseAll}>Collapse all</button>
        </div>
      {/if}
    </div>
  </div>

  {#each projectTrees as projectTree}
    <div class="project-group">
      <button class="project-header" onclick={() => toggleProject(projectTree.projectID)}>
        <ChevronRightOutline class={`fold-chevron ${!collapsedProjects[projectTree.projectID] ? 'expanded' : ''}`} size="sm" />
        <span>{displayProjectLabel(projectTree.projectID)}</span>
      </button>
      {#if !collapsedProjects[projectTree.projectID]}
        <div class="project-docs">
          {#each projectTree.lineages as lineage}
            <div class="lineage-group">
              <div class="lineage-root-row">
                {#if lineage.children.length > 0}
                  <button class="lineage-fold" onclick={() => toggleLineage(lineage.key)} aria-label="Toggle generated documents">
                    <ChevronRightOutline class={`fold-chevron lineage-chevron ${!collapsedLineages[lineage.key] ? 'expanded' : ''}`} size="xs" />
                  </button>
                {/if}
                <div class="doc-entry" data-doc-menu-host={lineage.root.id}>
                  <button
                    class="doc-item root-doc"
                    class:active={selectedDocId === lineage.root.id}
                    onclick={() => selectDocumentFromSidebar(lineage.root)}
                  >
                    <span class="doc-label">{displayTitle(lineage.root)}</span>
                    {#if docMetaSuffix(lineage.root)}
                      <span class="doc-meta">{docMetaSuffix(lineage.root)}</span>
                    {/if}
                  </button>

                  <button
                    class="doc-menu-trigger"
                    class:visible={activeDocMenuID === lineage.root.id}
                    aria-label="Document actions"
                    onclick={(event) => {
                      event.stopPropagation();
                      toggleDocMenu(lineage.root.id);
                    }}
                  >
                    ⋮
                  </button>

                  {#if activeDocMenuID === lineage.root.id}
                    <div class="doc-action-menu">
                      <button class="doc-action-item" onclick={() => runDocAction('build', lineage.root)}>Build</button>
                      {#if tasksEnabled}
                        <button class="doc-action-item" onclick={() => runDocAction('task', lineage.root)}>Task</button>
                      {/if}
                      <button class="doc-action-item" onclick={() => runDocAction('archive', lineage.root)}>Archive</button>
                      <button class="doc-action-item danger" onclick={() => runDocAction('delete', lineage.root)}>Delete</button>
                    </div>
                  {/if}
                </div>
              </div>
              {#if lineage.children.length > 0 && !collapsedLineages[lineage.key]}
                <div class="lineage-children">
                  {#each lineage.children as child}
                    <div class="doc-entry" data-doc-menu-host={child.id}>
                      <button
                        class="doc-item child-doc"
                        class:active={selectedDocId === child.id}
                        onclick={() => selectDocumentFromSidebar(child)}
                      >
                        <span class="doc-label">{displayTitle(child)}</span>
                        {#if docMetaSuffix(child)}
                          <span class="doc-meta">{docMetaSuffix(child)}</span>
                        {/if}
                      </button>

                      <button
                        class="doc-menu-trigger"
                        class:visible={activeDocMenuID === child.id}
                        aria-label="Document actions"
                        onclick={(event) => {
                          event.stopPropagation();
                          toggleDocMenu(child.id);
                        }}
                      >
                        ⋮
                      </button>

                      {#if activeDocMenuID === child.id}
                        <div class="doc-action-menu">
                          <button class="doc-action-item" onclick={() => runDocAction('build', child)}>Build</button>
                          {#if tasksEnabled}
                            <button class="doc-action-item" onclick={() => runDocAction('task', child)}>Task</button>
                          {/if}
                          <button class="doc-action-item" onclick={() => runDocAction('archive', child)}>Archive</button>
                          <button class="doc-action-item danger" onclick={() => runDocAction('delete', child)}>Delete</button>
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
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
    background: var(--surface-2);
    overflow-y: auto;
    padding: 20px 0;
    display: flex;
    flex-direction: column;
    gap: 24px;
    border-right: 1px solid var(--border-subtle);
    box-shadow: var(--inner-highlight);
  }

  .sidebar-controls {
    padding: 0 16px;
    display: flex;
    align-items: center;
    gap: 6px;
    position: relative;
    justify-content: space-between;
  }

  .sidebar-search {
    flex: 1;
    min-width: 0;
    border-radius: 0;
    padding: 0 0.55rem;
  }

  .options-wrap {
    position: relative;
    flex: 0 0 auto;
  }

  .options-trigger {
    border: 1px solid var(--border-subtle);
    background: var(--surface-1);
    color: #64748b;
    border-radius: 0.35rem;
    width: 1.9rem;
    height: 1.75rem;
    font-size: 0.9rem;
    line-height: 1;
    cursor: pointer;
  }

  .options-trigger:hover {
    color: #0f172a;
    background: var(--surface-2);
  }

  .options-menu {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    border: 1px solid var(--border-subtle);
    border-radius: 0.45rem;
    background: var(--surface-3);
    min-width: 13rem;
    display: flex;
    flex-direction: column;
    padding: 0.25rem;
    z-index: 12;
    box-shadow: var(--elevation-2), var(--inner-highlight);
  }

  .options-divider {
    height: 1px;
    margin: 0.25rem 0.2rem;
    background: var(--border-subtle);
  }

  .options-item {
    border: none;
    text-align: left;
    background: transparent;
    color: #334155;
    font-size: 0.72rem;
    padding: 0.38rem 0.5rem;
    border-radius: 0.3rem;
    cursor: pointer;
  }

  .options-item:hover {
    background: rgba(148, 163, 184, 0.18);
  }

  .project-header {
    background: transparent;
    border: none;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.56rem;
    text-transform: uppercase;
    letter-spacing: 0.14em;
    color: #64748b;
    font-weight: 800;
    margin-bottom: 8px;
    padding-left: 20px;
    cursor: pointer;
    text-align: left;
  }

  :global(.fold-chevron) {
    color: #64748b;
    transform: rotate(0deg);
    transition: transform 140ms ease, color 140ms ease;
  }

  :global(.fold-chevron.expanded) {
    transform: rotate(90deg);
    color: #334155;
  }

  .project-docs {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .lineage-group {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .lineage-root-row {
    display: flex;
    align-items: center;
    padding-left: 12px;
  }

  .doc-entry {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
  }

  .lineage-fold {
    background: transparent;
    border: none;
    color: #64748b;
    width: 16px;
    cursor: pointer;
    padding: 0;
    margin-right: 2px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  :global(.lineage-chevron) {
    color: #64748b;
  }

  .lineage-children {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin-left: 24px;
  }

  .doc-item {
    background: transparent;
    border: none;
    border-left: 2px solid transparent;
    display: flex;
    align-items: center;
    padding: 7px 12px;
    color: #334155;
    font-size: 0.73rem;
    font-weight: 650;
    text-transform: none;
    letter-spacing: 0.01em;
    text-align: left;
    cursor: pointer;
    transition: all 0.1s;
  }

  .root-doc {
    flex: 1;
    padding-left: 6px;
    padding-right: 30px;
  }

  .child-doc {
    padding-left: 12px;
    padding-right: 30px;
    border-left: 1px dashed var(--border-subtle);
    font-size: 0.69rem;
    width: 100%;
  }

  .doc-menu-trigger {
    position: absolute;
    right: 6px;
    top: 50%;
    transform: translateY(-50%);
    width: 1.2rem;
    height: 1.2rem;
    border: 1px solid transparent;
    background: transparent;
    color: #64748b;
    border-radius: 0.3rem;
    font-size: 0.8rem;
    line-height: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    cursor: pointer;
    z-index: 3;
  }

  .doc-menu-trigger.visible,
  .doc-entry:hover .doc-menu-trigger {
    opacity: 1;
  }

  .doc-menu-trigger:hover {
    background: rgba(148, 163, 184, 0.18);
    border-color: var(--border-subtle);
    color: #0f172a;
  }

  .doc-action-menu {
    position: absolute;
    right: 6px;
    top: calc(100% + 2px);
    min-width: 7.2rem;
    border: 1px solid var(--border-subtle);
    background: var(--surface-3);
    border-radius: 0.4rem;
    padding: 0.25rem;
    display: grid;
    gap: 0.1rem;
    box-shadow: var(--elevation-2), var(--inner-highlight);
    z-index: 11;
  }

  .doc-action-item {
    border: 0;
    background: transparent;
    color: #334155;
    font-size: 0.66rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    text-align: left;
    padding: 0.32rem 0.4rem;
    border-radius: 0.3rem;
    cursor: pointer;
  }

  .doc-action-item:hover {
    background: rgba(148, 163, 184, 0.16);
  }

  .doc-action-item.danger {
    color: #b91c1c;
  }

  .doc-meta {
    color: #64748b;
    font-size: 0.58rem;
    margin-left: 6px;
    letter-spacing: 0.02em;
  }

  .doc-item:hover {
    background: rgba(148, 163, 184, 0.14);
    color: #0f172a;
  }

  .doc-item.active {
    background: rgba(134, 188, 37, 0.16);
    color: #4d7c0f;
    border-left-color: #86BC25;
  }

  .empty-sidebar {
    padding: 40px 20px;
    text-align: center;
    color: #64748b;
    font-size: 0.85rem;
  }

  :global(.dark .options-trigger) {
    color: #9ca3af;
  }

  :global(.dark .options-trigger:hover) {
    color: #e2e8f0;
  }

  :global(.dark .options-item),
  :global(.dark .doc-item),
  :global(.dark .doc-meta),
  :global(.dark .project-header),
  :global(.dark .empty-sidebar),
  :global(.dark .fold-chevron),
  :global(.dark .lineage-chevron),
  :global(.dark .lineage-fold) {
    color: #94a3b8;
  }

  :global(.dark .doc-item:hover) {
    background: rgba(255, 255, 255, 0.05);
    color: #f8fafc;
  }

  :global(.dark .doc-item.active) {
    background: rgba(134, 188, 37, 0.07);
    color: #a3e635;
  }

  :global(.dark .doc-menu-trigger) {
    color: #94a3b8;
  }

  :global(.dark .doc-menu-trigger:hover) {
    color: #f8fafc;
  }

  :global(.dark .doc-action-item) {
    color: #d1d5db;
  }

  :global(.dark .doc-action-item.danger) {
    color: #fca5a5;
  }
</style>
