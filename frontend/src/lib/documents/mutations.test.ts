import { beforeEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  deleteJSON: vi.fn(),
  postJSON: vi.fn(),
  requestJSON: vi.fn()
}));

vi.mock('$lib/api', () => ({
  deleteJSON: apiMocks.deleteJSON,
  postJSON: apiMocks.postJSON,
  requestJSON: apiMocks.requestJSON
}));

import { buildDocument, deleteDocument, saveDocumentDraft, toggleDocumentArchive } from '$lib/documents/mutations';

describe('document mutations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('creates a new document when no selected ID is provided', async () => {
    await saveDocumentDraft(null, {
      title: 'Draft',
      content: 'Hello',
      projectID: 'project-1'
    });

    expect(apiMocks.postJSON).toHaveBeenCalledWith('/v1/documents', {
      project_id: 'project-1',
      title: 'Draft',
      content: 'Hello',
      format: 'markdown',
      status: 'active'
    });
  });

  it('updates an existing document when selected ID is present', async () => {
    await saveDocumentDraft('doc-1', {
      title: 'Draft',
      content: 'Updated',
      projectID: 'project-1'
    });

    expect(apiMocks.requestJSON).toHaveBeenCalledWith('/v1/documents/doc-1', 'PUT', {
      title: 'Draft',
      content: 'Updated',
      project_id: 'project-1'
    });
  });

  it('builds and deletes documents through the API helpers', async () => {
    await buildDocument('doc-2');
    await deleteDocument('doc-2');

    expect(apiMocks.postJSON).toHaveBeenCalledWith('/v1/documents/doc-2/build', {});
    expect(apiMocks.deleteJSON).toHaveBeenCalledWith('/v1/documents/doc-2');
  });

  it('toggles archive status and returns the next state', async () => {
    await expect(toggleDocumentArchive('doc-3', { status: 'active' })).resolves.toBe('archived');
    expect(apiMocks.requestJSON).toHaveBeenCalledWith('/v1/documents/doc-3', 'PUT', { status: 'archived' });
  });
});
