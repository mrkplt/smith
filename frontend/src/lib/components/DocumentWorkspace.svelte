<script lang="ts">
  import EmptyState from '$lib/components/EmptyState.svelte';
  import DocumentEditorPane from '$lib/components/DocumentEditorPane.svelte';
  import DocumentPreviewPane from '$lib/components/DocumentPreviewPane.svelte';
  import DocumentWorkspaceHeader from '$lib/components/DocumentWorkspaceHeader.svelte';
  import { marked } from 'marked';

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
    return marked.parse(editContent || '') as string;
  });
</script>

<main class="doc-editor-pane">
  {#if isEditing || selectedDocId || editTitle}
    <div class="editor-frame">
      <DocumentWorkspaceHeader
        {isEditing}
        {editTitle}
        {editProjectID}
        {projects}
        {onEditTitle}
        {onEditProjectID}
        {onStartEdit}
        {onSaveDocument}
        {onCancelEdit}
        {onBuildDoc}
        {onArchiveDoc}
        {onDeleteDoc}
      />

      <div class="editor-viewport flex-1 flex">
        {#if isEditing}
          <div class="flex w-full h-full divide-x divide-gray-900 overflow-hidden">
            <DocumentEditorPane {editContent} {onEditContent} />
            <DocumentPreviewPane {renderedContent} showHeader={true} />
          </div>
        {:else}
          <DocumentPreviewPane {renderedContent} />
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
  .editor-viewport {
    flex: 1;
    overflow: hidden;
    display: flex;
  }
</style>
