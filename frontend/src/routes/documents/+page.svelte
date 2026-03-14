<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { deleteJSON, requestJSON, postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DocChatModal from '$lib/components/DocChatModal.svelte';
	import DocumentsSidebar from '$lib/components/DocumentsSidebar.svelte';
	import DocumentWorkspace from '$lib/components/DocumentWorkspace.svelte';
  import { Checkbox, Button } from 'flowbite-svelte';
  import { PlusOutline, MessagesOutline } from 'flowbite-svelte-icons';

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
		if (!selectedDocId) {
			try {
				await postJSON("/v1/documents", {
					project_id: editProjectID,
					title: editTitle,
					content: editContent,
					format: "markdown",
					status: "active"
				});
				pushToast("Document created", "ok");
				isEditing = false;
			} catch (err: any) {
				pushToast(err.message, "err");
			}
			return;
		}

		try {
			await requestJSON(`/v1/documents/${selectedDocId}`, "PUT", {
				title: editTitle,
				content: editContent,
				project_id: editProjectID
			});
			pushToast("Document saved", "ok");
			isEditing = false;
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function buildDoc() {
		if (!selectedDocId) return;
		try {
			await postJSON(`/v1/documents/${selectedDocId}/build`, {});
			pushToast("Build loop started", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function archiveDoc() {
		if (!selectedDocId || !selectedDoc) return;
		const nextStatus = selectedDoc.status === 'active' ? 'archived' : 'active';
		try {
			await requestJSON(`/v1/documents/${selectedDocId}`, "PUT", { status: nextStatus });
			pushToast(`Document ${nextStatus}`, "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function deleteDoc() {
		if (!selectedDocId || !confirm("Delete document?")) return;
		try {
			await deleteJSON(`/v1/documents/${selectedDocId}`);
			selectedDocId = null;
			pushToast("Document deleted", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
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
    <div class="flex items-center gap-4">
      <Checkbox bind:checked={showAll} class="text-gray-400 font-medium uppercase text-[10px] tracking-widest">Show Archived</Checkbox>
    </div>
  {/snippet}
</TopBar>

<!-- Inline Actions Header -->
<div class="flex justify-end gap-2 -mt-14 mb-8 relative z-50 px-4">
  <Button color="alternative" class="bg-black border-gray-800 text-[#86BC25] hover:bg-white/5 rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={() => chatOpen = true}>
    <MessagesOutline size="xs" class="mr-1.5" />
    Draft with AI
  </Button>
  
  <Button color="alternative" class="bg-[#86BC25] text-black rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={createNew}>
    <PlusOutline size="xs" class="mr-1.5" />
    New Doc
  </Button>
</div>

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
