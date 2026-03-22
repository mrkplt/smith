<script lang="ts">
  import EmptyState from '$lib/components/EmptyState.svelte';
  import DocumentEditorPane from '$lib/components/DocumentEditorPane.svelte';
  import DocumentPreviewPane from '$lib/components/DocumentPreviewPane.svelte';
  import PRDValidationPanel from '$lib/components/PRDValidationPanel.svelte';
  import DocumentWorkspaceHeader from '$lib/components/DocumentWorkspaceHeader.svelte';
  import type { PRDValidationDiagnostic, PRDValidationReport } from '$lib/documents/prd-validation';
  import { marked } from 'marked';

  interface Props {
    isEditing: boolean;
    selectedDocId: string | null;
    selectedDoc: any;
    editTitle: string;
    editContent: string;
    editProjectID: string;
    editFormat: 'markdown' | 'json';
    validationReport: PRDValidationReport | null;
    validationBusy: boolean;
    validationError: string;
    chatEnabled: boolean;
    tasksEnabled: boolean;
    resolveDiagnosticEnabled: boolean;
    projects: any[];
    onEditTitle: (value: string) => void;
    onEditContent: (value: string) => void;
    onEditProjectID: (value: string) => void;
    onEditFormat: (value: 'markdown' | 'json') => void;
    onFocusContextChange: (focus: {
      lineIndex: number | null;
      sectionId: string;
      selectionText: string;
    }) => void;
    onStartEdit: () => void;
    onSaveDocument: () => void;
    onCancelEdit: () => void;
    onRefreshValidation: () => void;
    onRefineWithAI: () => void;
    onResolveDiagnostic: (diagnostic: PRDValidationDiagnostic) => void;
  }

  let {
    isEditing,
    selectedDocId,
    selectedDoc,
    editTitle,
    editContent,
    editProjectID,
    editFormat,
    validationReport,
    validationBusy,
    validationError,
    chatEnabled,
    tasksEnabled,
    resolveDiagnosticEnabled,
    projects,
    onEditTitle,
    onEditContent,
    onEditProjectID,
    onEditFormat,
    onFocusContextChange,
    onStartEdit,
    onSaveDocument,
    onCancelEdit,
    onRefreshValidation,
    onRefineWithAI,
    onResolveDiagnostic
  }: Props = $props();

  function escapeHTML(content: string): string {
    return content
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#39;');
  }

  const renderedContent = $derived.by(() => {
    if (editFormat === 'json') {
      return `<pre>${escapeHTML(editContent || '')}</pre>`;
    }
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
        {editFormat}
        {projects}
        {onEditTitle}
        {onEditProjectID}
        {onEditFormat}
        {onStartEdit}
        {onSaveDocument}
        {onCancelEdit}
      />

      <PRDValidationPanel
        report={validationReport}
        busy={validationBusy}
        errorMessage={validationError}
        {chatEnabled}
        resolveDiagnosticEnabled={resolveDiagnosticEnabled}
        onRecheck={onRefreshValidation}
        onRefineWithAI={onRefineWithAI}
        {onResolveDiagnostic}
      />

      <div class="editor-viewport flex-1 flex">
        {#if isEditing}
          <DocumentEditorPane {editContent} format={editFormat} {onEditContent} {onFocusContextChange} />
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
    background: var(--surface-1);
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
    background: var(--surface-1);
  }
</style>
