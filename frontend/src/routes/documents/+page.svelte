<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { createTaskContract, fetchJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DocChatModal from '$lib/components/DocChatModal.svelte';
	import DocumentsSidebar from '$lib/components/DocumentsSidebar.svelte';
	import DocumentWorkspace from '$lib/components/DocumentWorkspace.svelte';
	import DocumentsPageActions from '$lib/components/DocumentsPageActions.svelte';
	import { isChatEnabled, isPRDDiagnosticResolveEnabled, isTasksEnabled } from '$lib/feature-flags';
	import { buildDocument, deleteDocument, saveDocumentDraft, toggleDocumentArchive } from '$lib/documents/mutations';
	import {
		inferPRDFormat,
		validatePRDContent,
		type PRDFormat,
		type PRDValidationDiagnostic,
		type PRDValidationReport
	} from '$lib/documents/prd-validation';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { tick } from 'svelte';

	let showAll = $state(false);
	let docSearchQuery = $state('');
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
	let linkedDocRef = $state<string | null>(null);
	let linkedDocResolved = $state(false);
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
		const query = docSearchQuery.trim().toLowerCase();
		$appState.documents.forEach((d: any) => {
			if (!showAll && d.status === 'archived') return;
			if (query) {
				const title = String(d.title || '').toLowerCase();
				const id = String(d.id || '').toLowerCase();
				const sourceRef = String(d.source_ref || '').toLowerCase();
				if (!title.includes(query) && !id.includes(query) && !sourceRef.includes(query)) {
					return;
				}
			}
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
const tasksFeatureEnabled = $derived(isTasksEnabled());

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

	function documentMatchesReference(doc: any, reference: string): boolean {
		const normalized = String(reference || '').trim();
		if (!normalized) {
			return false;
		}
		const docID = String(doc?.id || '').trim();
		const docRef = String(doc?.source_ref || '').trim();
		if (docID && normalized === docID) {
			return true;
		}
		if (docRef && normalized === docRef) {
			return true;
		}
		if (docID && normalized === `doc:${docID}`) {
			return true;
		}
		return false;
	}

	function findDocumentByReference(reference: string): any | null {
		for (const doc of $appState.documents) {
			if (documentMatchesReference(doc, reference)) {
				return doc;
			}
		}
		return null;
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
		linkedDocResolved = true;
	}

	function clearActiveDocument() {
		selectedDocId = null;
		isEditing = false;
		editorFocus = { lineIndex: null, sectionId: '', selectionText: '' };
	}

	$effect(() => {
		const requested = String(page.url.searchParams.get('doc') || '').trim();
		if (!requested) {
			linkedDocRef = null;
			linkedDocResolved = false;
			return;
		}
		linkedDocRef = requested;
		linkedDocResolved = false;
	});

	$effect(() => {
		const targetRef = linkedDocRef;
		if (!targetRef || linkedDocResolved) {
			return;
		}
		const matched = findDocumentByReference(targetRef);
		if (matched) {
			selectDocument(matched);
			return;
		}
		if ($appState.documents.length > 0) {
			pushToast('Requested document was not found (it may be archived or deleted).', 'err');
			linkedDocResolved = true;
		}
	});

	function orderedVisibleDocuments(documents: any[]): any[] {
		const grouped: Record<string, any[]> = {};
		for (const doc of documents) {
			if (!showAll && doc.status === 'archived') {
				continue;
			}
			const projectID = String(doc.project_id || 'default');
			if (!grouped[projectID]) {
				grouped[projectID] = [];
			}
			grouped[projectID].push(doc);
		}
		const orderedProjectIDs = Object.keys(grouped).sort();
		const ordered: any[] = [];
		for (const projectID of orderedProjectIDs) {
			ordered.push(...grouped[projectID]);
		}
		return ordered;
	}

	function nextActiveDocumentAfterMutation(previousDocs: any[], nextDocs: any[], currentDocID: string): any | null {
		const prevVisible = orderedVisibleDocuments(previousDocs);
		const nextVisible = orderedVisibleDocuments(nextDocs);
		if (nextVisible.length === 0) {
			return null;
		}
		const prevIndex = prevVisible.findIndex((doc) => String(doc.id) === currentDocID);
		if (prevIndex < 0) {
			return nextVisible[0];
		}
		return nextVisible[prevIndex] || nextVisible[Math.max(0, prevIndex - 1)] || nextVisible[0] || null;
	}

	async function reloadDocumentsFromAPI(): Promise<any[]> {
		const payload = await fetchJSON('/v1/documents');
		const docs = Array.isArray(payload) ? payload : [];
		appState.update((state) => ({ ...state, documents: docs }));
		return docs;
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
			const savedDoc = await saveDocumentDraft(selectedDocId, {
				title: editTitle,
				content: editContent,
				projectID: editProjectID,
				format: editFormat
			});
			if (savedDoc?.id) {
				const savedID = String(savedDoc.id);
				appState.update((state) => {
					const nextDocuments = [...state.documents];
					const existingIndex = nextDocuments.findIndex((doc: any) => doc.id === savedID);
					if (existingIndex >= 0) {
						nextDocuments[existingIndex] = { ...nextDocuments[existingIndex], ...savedDoc };
					} else {
						nextDocuments.unshift(savedDoc);
					}
					return { ...state, documents: nextDocuments };
				});
				selectedDocId = savedID;
				editProjectID = String(savedDoc.project_id || editProjectID || '');
			}
			pushToast(selectedDocId ? "Document saved" : "Document created", "ok");
			isEditing = false;
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function buildDoc(doc?: any) {
		const targetDocId = String(doc?.id || selectedDocId || '').trim();
		if (!targetDocId) {
			return;
		}
		if (doc && String(selectedDocId || '') !== targetDocId) {
			selectDocument(doc);
		}
		try {
			await buildDocument(targetDocId);
			pushToast("Build loop started", "ok");
		} catch (err: any) {
			const report = err?.body?.report;
			if (report) {
				validationReport = report;
				validationError = '';
				const issueCount = Number((report?.errors || []).length) + Number((report?.warnings || []).length);
				pushToast(`Build blocked by PRD readiness (${issueCount} issue${issueCount === 1 ? '' : 's'}).`, "err");
				return;
			}
			pushToast(err.message, "err");
		}
	}

	async function archiveDoc(doc?: any) {
		const targetDoc = doc || selectedDoc;
		const docID = String(targetDoc?.id || selectedDocId || '').trim();
		if (!docID || !targetDoc) {
			return;
		}
		if (doc && String(selectedDocId || '') !== docID) {
			selectDocument(doc);
		}
		const previousDocs = [...$appState.documents];
		try {
			const nextStatus = await toggleDocumentArchive(docID, targetDoc);
			const freshDocs = await reloadDocumentsFromAPI();
			let nextActiveDoc: any | null = null;
			if (nextStatus === 'archived') {
				nextActiveDoc = nextActiveDocumentAfterMutation(previousDocs, freshDocs, docID);
			} else {
				nextActiveDoc = freshDocs.find((doc: any) => String(doc.id) === docID) || null;
			}
			if (nextActiveDoc) {
				selectDocument(nextActiveDoc);
			} else {
				clearActiveDocument();
			}
			pushToast(`Document ${nextStatus}`, "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function deleteDoc(doc?: any) {
		const targetDoc = doc || selectedDoc;
		const docID = String(targetDoc?.id || selectedDocId || '').trim();
		if (!docID) {
			return;
		}
		const targetLabel = String(targetDoc?.title || '').trim();
		const deletePrompt = targetLabel
			? `Delete "${targetLabel}"? This action is permanent and cannot be undone.`
			: 'Delete this document? This action is permanent and cannot be undone.';
		if (!confirm(deletePrompt)) {
			return;
		}
		if (doc && String(selectedDocId || '') !== docID) {
			selectDocument(doc);
		}
		const previousDocs = [...$appState.documents];
		try {
			await deleteDocument(docID);
			const freshDocs = await reloadDocumentsFromAPI();
			const nextActiveDoc = nextActiveDocumentAfterMutation(previousDocs, freshDocs, docID);
			if (nextActiveDoc) {
				selectDocument(nextActiveDoc);
			} else {
				clearActiveDocument();
			}
			pushToast("Document deleted", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function createTaskFromDocument(doc?: any) {
		if (!tasksFeatureEnabled) {
			pushToast('Task contracts are currently gated and unavailable in this environment.', 'err');
			return;
		}
		const targetDoc = doc || selectedDoc;
		if (!targetDoc) {
			pushToast('select a document first', 'err');
			return;
		}
		if (doc && String(selectedDocId || '') !== String(doc.id || '')) {
			selectDocument(doc);
		}
		try {
			const providerProfileID = 'codex-default';
			const projectID = String(targetDoc.project_id || $appState.projects[0]?.id || 'smith');
			const objective = String(targetDoc.title || 'Document-derived task').trim();
			const sourceDocument = String(targetDoc.source_ref || `doc:${targetDoc.id}`);
			const validation = ['go test ./...'];
			const created = await createTaskContract({
				project_id: projectID,
				provider_profile_id: providerProfileID,
				source_document: sourceDocument,
				objective,
				validation,
				metadata: {
					document_id: String(targetDoc.id || ''),
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
    <DocumentsPageActions
		onCreateNew={createNew}
		onImportPRD={handleImportPRD}
	/>
  {/snippet}
</TopBar>

<div class="doc-layout" style={`grid-template-columns: ${docLayoutColumns};`}>
	<DocumentsSidebar
		{projectIDs}
		{projectsWithDocs}
		{selectedDocId}
		{showAll}
		tasksEnabled={tasksFeatureEnabled}
		searchQuery={docSearchQuery}
		onShowAllChange={(value) => showAll = value}
		onSearchQueryChange={(value) => docSearchQuery = value}
		onSelectDocument={selectDocument}
		onBuildDocument={(doc) => void buildDoc(doc)}
		onCreateTaskFromDocument={(doc) => void createTaskFromDocument(doc)}
		onArchiveDocument={(doc) => void archiveDoc(doc)}
		onDeleteDocument={(doc) => void deleteDoc(doc)}
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
		tasksEnabled={tasksFeatureEnabled}
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
		height: calc(100vh - 172px);
		background: var(--surface-1);
		overflow: hidden;
		margin-top: 10px;
		margin-inline: 1rem;
		border: 1px solid var(--border-subtle);
		box-shadow: var(--elevation-1), var(--inner-highlight);
		border-radius: 10px;
	}

</style>
