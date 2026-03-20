<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { createTaskContract } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DocChatModal from '$lib/components/DocChatModal.svelte';
	import DocumentsSidebar from '$lib/components/DocumentsSidebar.svelte';
	import DocumentWorkspace from '$lib/components/DocumentWorkspace.svelte';
	import DocumentsPageActions from '$lib/components/DocumentsPageActions.svelte';
	import { isChatEnabled, isPRDDiagnosticResolveEnabled } from '$lib/feature-flags';
	import { buildDocument, deleteDocument, saveDocumentDraft, toggleDocumentArchive } from '$lib/documents/mutations';
	import {
		inferPRDFormat,
		validatePRDContent,
		type PRDFormat,
		type PRDValidationDiagnostic,
		type PRDValidationReport
	} from '$lib/documents/prd-validation';
	import { goto } from '$app/navigation';
	import { tick } from 'svelte';

	let showAll = $state(false);
	let chatOpen = $state(false);
	let isEditing = $state(false);
	
	let selectedDocId = $state<string | null>(null);
	let editTitle = $state("");
	let editContent = $state("");
	let editProjectID = $state("");
	let editFormat = $state<PRDFormat>('markdown');
	let validationReport = $state<PRDValidationReport | null>(null);
	let validationBusy = $state(false);
	let validationError = $state('');
	let validationRequestSeq = 0;
	let editorFocus = $state<{ lineIndex: number | null; sectionId: string; selectionText: string }>({
		lineIndex: null,
		sectionId: '',
		selectionText: ''
	});
	let chatSeed = $state<{
		title: string;
		content: string;
		format: PRDFormat;
		validationReport: PRDValidationReport | null;
		targetedDiagnostic?: PRDValidationDiagnostic;
		documentId?: string;
		documentVersion?: string;
		documentStatus?: string;
		projectId?: string;
	} | null>(null);

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
const docLayoutColumns = $derived(
	chatOpen
		? '260px minmax(0, 1fr) minmax(440px, 560px)'
		: '260px minmax(0, 1fr)'
);
const diagnosticResolveEnabled = $derived(isPRDDiagnosticResolveEnabled());
const chatFeatureEnabled = $derived(isChatEnabled());

	const guidepostFocusContext = $derived.by(() => {
		const selected = selectedDoc;
		return {
			surface: 'document_editor',
			entityType: 'document',
			entitySubtype: 'prd',
			entityId: selected?.id ? String(selected.id) : 'draft',
			entityVersion: selected?.updated_at ? String(selected.updated_at) : '',
			sectionId: editorFocus.sectionId,
			selectionText: editorFocus.selectionText,
			lineIndex: editorFocus.lineIndex,
			uiState: {
				activePane: chatOpen ? 'Guidepost' : 'Editor',
				centerTab: 'document'
			}
		};
	});

	function inferDocumentFormat(doc: any): PRDFormat {
		const explicit = String(doc?.format || '').trim().toLowerCase();
		if (explicit === 'json') {
			return 'json';
		}
		if (explicit === 'markdown' || explicit === 'md') {
			return 'markdown';
		}
		return inferPRDFormat(String(doc?.content || ''));
	}

	function selectDocument(doc: any) {
		const nextFormat = inferDocumentFormat(doc);
		selectedDocId = doc.id;
		editTitle = doc.title;
		editContent = doc.content;
		editProjectID = doc.project_id;
		editFormat = nextFormat;
		isEditing = false;
		editorFocus = { lineIndex: null, sectionId: '', selectionText: '' };
		void refreshValidation(doc.content, nextFormat);
	}

	function startEdit() {
		isEditing = true;
	}

  function cancelEdit() {
    isEditing = false;
	editorFocus = { lineIndex: null, sectionId: '', selectionText: '' };
  }

	async function saveDocument() {
		try {
			await saveDocumentDraft(selectedDocId, {
				title: editTitle,
				content: editContent,
				projectID: editProjectID,
				format: editFormat
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
		editFormat = 'markdown';
		isEditing = true;
		validationReport = null;
		validationError = '';
		validationBusy = false;
		editorFocus = { lineIndex: null, sectionId: '', selectionText: '' };
	}

	function handleDraftFinalized(title: string, content: string) {
		editTitle = title;
		editContent = content;
		editProjectID = $appState.projects[0]?.id || "";
		editFormat = 'markdown';
		selectedDocId = null;
		isEditing = true;
		chatOpen = false;
		editorFocus = { lineIndex: null, sectionId: '', selectionText: '' };
		void refreshValidation(content, 'markdown');
	}

	function handleEditorFocusContext(next: { lineIndex: number | null; sectionId: string; selectionText: string }) {
		editorFocus = next;
	}

	function titleFromFilename(filename: string): string {
		const withoutExtension = filename.replace(/\.[^/.]+$/, '');
		const normalized = withoutExtension.replace(/[_-]+/g, ' ').trim();
		if (normalized === '') {
			return 'Imported PRD';
		}
		return normalized.replace(/\b\w/g, (char) => char.toUpperCase());
	}

	async function handleImportPRD(file: File) {
		try {
			const raw = await file.text();
			if (raw.trim() === '') {
				pushToast('Uploaded file is empty.', 'err');
				return;
			}

			const lowerName = file.name.toLowerCase();
			const importedFormat: PRDFormat = lowerName.endsWith('.json') ? 'json' : inferPRDFormat(raw);
			let nextFormat: PRDFormat = importedFormat;
			let nextContent = raw;
			let nextReport: PRDValidationReport | null = null;

			try {
				const validation = await validatePRDContent(raw, importedFormat);
				nextReport = validation.report;
				validationError = '';
				if (importedFormat === 'json' && validation.report.valid && validation.canonical_markdown) {
					nextFormat = 'markdown';
					nextContent = validation.canonical_markdown;
					pushToast('Imported JSON PRD and converted it to markdown.', 'ok');
				} else if (!validation.report.valid) {
					pushToast('Imported PRD has readiness issues. Follow suggestions below.', 'muted');
				} else {
					pushToast('PRD imported successfully.', 'ok');
				}
			} catch (err: any) {
				validationError = err.message || 'Failed to validate imported PRD';
				pushToast(validationError, 'err');
			}

			editTitle = titleFromFilename(file.name);
			editContent = nextContent;
			editProjectID = $appState.projects[0]?.id || editProjectID || '';
			editFormat = nextFormat;
			selectedDocId = null;
			isEditing = true;
			validationReport = nextReport;
		} catch (err: any) {
			pushToast(err.message || 'Failed to read uploaded file', 'err');
		}
	}

	async function openChatWithSeed(seed: {
		title: string;
		content: string;
		format: PRDFormat;
		validationReport: PRDValidationReport | null;
		targetedDiagnostic?: PRDValidationDiagnostic;
		documentId?: string;
		documentVersion?: string;
		documentStatus?: string;
		projectId?: string;
	} | null) {
		if (!chatFeatureEnabled) {
			pushToast('Chat is disabled in this environment.', 'err');
			return;
		}
		chatSeed = seed;
		if (chatOpen) {
			chatOpen = false;
			await tick();
		}
		chatOpen = true;
	}

	function refineWithAI() {
		if (!chatFeatureEnabled) {
			pushToast('Chat is disabled in this environment.', 'err');
			return;
		}
		if (editContent.trim() === '') {
			pushToast('Add or upload PRD content before requesting refinement.', 'err');
			return;
		}
		const resolvedProjectID = String(editProjectID || selectedDoc?.project_id || '').trim();
		pushToast('Opening AI refinement with your current draft and diagnostics.', 'muted');
		void openChatWithSeed({
			title: editTitle || 'Draft PRD',
			content: editContent,
			format: editFormat,
			validationReport,
			documentId: selectedDoc?.id ? String(selectedDoc.id) : undefined,
			documentVersion: selectedDoc?.updated_at ? String(selectedDoc.updated_at) : undefined,
			documentStatus: selectedDoc?.status ? String(selectedDoc.status) : undefined,
			projectId: resolvedProjectID === '' ? undefined : resolvedProjectID
		});
	}

	function runDiagnosticAgentAction(diagnostic: PRDValidationDiagnostic) {
		if (!chatFeatureEnabled) {
			pushToast('Chat is disabled in this environment.', 'err');
			return;
		}
		if (editContent.trim() === '') {
			pushToast('Add PRD content before running AI actions.', 'err');
			return;
		}
		const resolvedProjectID = String(editProjectID || selectedDoc?.project_id || '').trim();
		pushToast(`Resolving ${diagnostic.code} with AI agent.`, 'muted');
		void openChatWithSeed({
			title: editTitle || 'Draft PRD',
			content: editContent,
			format: editFormat,
			validationReport,
			targetedDiagnostic: diagnostic,
			documentId: selectedDoc?.id ? String(selectedDoc.id) : undefined,
			documentVersion: selectedDoc?.updated_at ? String(selectedDoc.updated_at) : undefined,
			documentStatus: selectedDoc?.status ? String(selectedDoc.status) : undefined,
			projectId: resolvedProjectID === '' ? undefined : resolvedProjectID
		});
	}

	async function refreshValidation(content = editContent, format: PRDFormat = editFormat) {
		const trimmed = content.trim();
		if (trimmed === '') {
			validationReport = null;
			validationError = '';
			validationBusy = false;
			return;
		}

		const requestID = ++validationRequestSeq;
		validationBusy = true;
		try {
			const validation = await validatePRDContent(content, format);
			if (requestID !== validationRequestSeq) {
				return;
			}
			validationReport = validation.report;
			validationError = '';
		} catch (err: any) {
			if (requestID !== validationRequestSeq) {
				return;
			}
			validationError = err.message || 'Validation request failed';
			validationReport = null;
		} finally {
			if (requestID === validationRequestSeq) {
				validationBusy = false;
			}
		}
	}

	$effect(() => {
		const content = editContent;
		const format = editFormat;
		if (content.trim() === '') {
			validationReport = null;
			validationError = '';
			validationBusy = false;
			return;
		}
		const timer = setTimeout(() => {
			void refreshValidation(content, format);
		}, 450);
		return () => clearTimeout(timer);
	});
</script>

<TopBar title="Documents">
  {#snippet controls()}
    <div></div>
  {/snippet}
</TopBar>

<DocumentsPageActions
	{showAll}
	onShowAllChange={(value) => showAll = value}
	onCreateNew={createNew}
	onImportPRD={handleImportPRD}
/>

<div class="doc-layout" style={`grid-template-columns: ${docLayoutColumns};`}>
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
		{editFormat}
		{validationReport}
		{validationBusy}
		validationError={validationError}
		chatEnabled={chatFeatureEnabled}
		resolveDiagnosticEnabled={diagnosticResolveEnabled}
		projects={$appState.projects}
		onEditTitle={(value) => editTitle = value}
		onEditContent={(value) => editContent = value}
		onEditProjectID={(value) => editProjectID = value}
		onEditFormat={(value) => editFormat = value}
		onFocusContextChange={handleEditorFocusContext}
		onStartEdit={startEdit}
		onSaveDocument={saveDocument}
		onCancelEdit={cancelEdit}
		onRefreshValidation={refreshValidation}
		onRefineWithAI={refineWithAI}
		onResolveDiagnostic={runDiagnosticAgentAction}
		onBuildDoc={buildDoc}
		onCreateTask={createTaskFromDocument}
		onArchiveDoc={archiveDoc}
		onDeleteDoc={deleteDoc}
	/>

	{#if chatFeatureEnabled && chatOpen}
		<DocChatModal
			open={chatOpen}
			onClose={() => chatOpen = false}
			onDraftFinalized={handleDraftFinalized}
			seedDraft={chatSeed}
			focusContext={guidepostFocusContext}
		/>
	{/if}
</div>

<style>
	.doc-layout {
		display: grid;
		height: calc(100vh - 160px);
		background: #000000;
		overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.05);
	}

</style>
