<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { createTaskContract } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DocChatModal from '$lib/components/DocChatModal.svelte';
	import DocumentsSidebar from '$lib/components/DocumentsSidebar.svelte';
	import DocumentWorkspace from '$lib/components/DocumentWorkspace.svelte';
	import DocumentsPageActions from '$lib/components/DocumentsPageActions.svelte';
	import { buildDocument, deleteDocument, saveDocumentDraft, toggleDocumentArchive } from '$lib/documents/mutations';
	import { goto } from '$app/navigation';

	let showAll = $state(false);
	let chatOpen = $state(false);
	let isEditing = $state(false);
	
	let selectedDocId = $state<string | null>(null);
	let editTitle = $state("");
	let editContent = $state("");
	let editProjectID = $state("");

	const selectedDoc = $derived(
		$appState.documents.find((d: any) => d.id === selectedDocId) || null
	);

	const projectsWithDocs = $derived.by(() => {
		const grouped: Record<string, any[]> = {};
		$appState.documents.forEach((d: any) => {
			if (!showAll && d.status === 'archived') return;
			if (!grouped[d.project_id]) grouped[d.project_id] = [];
			grouped[d.project_id].push(d);
		});
		return grouped;
	});

	const projectIDs = $derived(Object.keys(projectsWithDocs).sort());

	function selectDocument(doc: any) {
		selectedDocId = doc.id;
		editTitle = doc.title;
		editContent = doc.content;
		editProjectID = doc.project_id;
		isEditing = false;
	}

	function startEdit() {
		isEditing = true;
	}

  function cancelEdit() {
    isEditing = false;
  }

	async function saveDocument() {
		try {
			await saveDocumentDraft(selectedDocId, {
				title: editTitle,
				content: editContent,
				projectID: editProjectID
			});
			pushToast(selectedDocId ? "Document saved" : "Document created", "ok");
			isEditing = false;
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function buildDoc() {
		if (!selectedDocId) return;
		try {
			await buildDocument(selectedDocId);
			pushToast("Build loop started", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function archiveDoc() {
		if (!selectedDocId || !selectedDoc) return;
		try {
			const nextStatus = await toggleDocumentArchive(selectedDocId, selectedDoc);
			pushToast(`Document ${nextStatus}`, "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function deleteDoc() {
		if (!selectedDocId || !confirm("Delete document?")) return;
		try {
			await deleteDocument(selectedDocId);
			selectedDocId = null;
			pushToast("Document deleted", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function createTaskFromDocument() {
		if (!selectedDoc) {
			pushToast('select a document first', 'err');
			return;
		}
		try {
			const providerProfileID = 'codex-default';
			const projectID = String(selectedDoc.project_id || $appState.projects[0]?.id || 'smith');
			const objective = String(selectedDoc.title || 'Document-derived task').trim();
			const sourceDocument = String(selectedDoc.source_ref || `doc:${selectedDoc.id}`);
			const validation = ['go test ./...'];
			const created = await createTaskContract({
				project_id: projectID,
				provider_profile_id: providerProfileID,
				source_document: sourceDocument,
				objective,
				validation,
				metadata: {
					document_id: String(selectedDoc.id || ''),
					document_title: objective,
					created_from: 'documents-page',
				},
				actor: 'operator',
			});
			pushToast(`task contract ${created.id} created`, 'ok');
			await goto('/tasks');
		} catch (err: any) {
			pushToast(err.message || 'failed to create task from document', 'err');
		}
	}

	function createNew() {
		selectedDocId = null;
		editTitle = "Untitled Document";
		editContent = "";
		editProjectID = $appState.projects[0]?.id || "";
		isEditing = true;
	}

	function handleDraftFinalized(title: string, content: string) {
		editTitle = title;
		editContent = content;
		editProjectID = $appState.projects[0]?.id || "";
		selectedDocId = null;
		isEditing = true;
		chatOpen = false;
	}
</script>

<TopBar title="Documents">
  {#snippet controls()}
    <div></div>
  {/snippet}
</TopBar>

<DocumentsPageActions
	{showAll}
	onShowAllChange={(value) => showAll = value}
	onOpenChat={() => chatOpen = true}
	onCreateNew={createNew}
/>

<DocChatModal
	open={chatOpen}
	onClose={() => chatOpen = false}
	onDraftFinalized={handleDraftFinalized}
/>

<div class="doc-layout">
	<DocumentsSidebar
		{projectIDs}
		{projectsWithDocs}
		{selectedDocId}
		onSelectDocument={selectDocument}
	/>

	<DocumentWorkspace
		{isEditing}
		{selectedDocId}
		{selectedDoc}
		{editTitle}
		{editContent}
		{editProjectID}
		projects={$appState.projects}
		onEditTitle={(value) => editTitle = value}
		onEditContent={(value) => editContent = value}
		onEditProjectID={(value) => editProjectID = value}
		onStartEdit={startEdit}
		onSaveDocument={saveDocument}
		onCancelEdit={cancelEdit}
		onBuildDoc={buildDoc}
		onCreateTask={createTaskFromDocument}
		onArchiveDoc={archiveDoc}
		onDeleteDoc={deleteDoc}
	/>
</div>

<style>
	.doc-layout {
		display: grid;
		grid-template-columns: 260px 1fr;
		height: calc(100vh - 160px);
		background: #000000;
		overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.05);
	}

</style>
