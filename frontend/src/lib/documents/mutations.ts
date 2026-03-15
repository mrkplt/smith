import { deleteJSON, postJSON, requestJSON } from '$lib/api';

/**
 * The editable fields for a document draft.
 */
export interface DocumentDraft {
  title: string;
  content: string;
  projectID: string;
}

/**
 * The subset of document data needed for archive toggles.
 */
export interface DocumentRecord {
  status: string;
}

/**
 * Persists a new or existing document draft.
 */
export async function saveDocumentDraft(selectedDocID: string | null, draft: DocumentDraft): Promise<void> {
  if (!selectedDocID) {
    await postJSON('/v1/documents', {
      project_id: draft.projectID,
      title: draft.title,
      content: draft.content,
      format: 'markdown',
      status: 'active'
    });
    return;
  }

  await requestJSON(`/v1/documents/${selectedDocID}`, 'PUT', {
    title: draft.title,
    content: draft.content,
    project_id: draft.projectID
  });
}

/**
 * Starts the document build loop for a saved document.
 */
export async function buildDocument(selectedDocID: string): Promise<void> {
  await postJSON(`/v1/documents/${selectedDocID}/build`, {});
}

/**
 * Toggles a document between active and archived states.
 */
export async function toggleDocumentArchive(selectedDocID: string, selectedDoc: DocumentRecord): Promise<string> {
  const nextStatus = selectedDoc.status === 'active' ? 'archived' : 'active';
  await requestJSON(`/v1/documents/${selectedDocID}`, 'PUT', { status: nextStatus });
  return nextStatus;
}

/**
 * Deletes a document by id.
 */
export async function deleteDocument(selectedDocID: string): Promise<void> {
  await deleteJSON(`/v1/documents/${selectedDocID}`);
}
