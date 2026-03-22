import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';

import DocumentsSidebar from './DocumentsSidebar.svelte';

describe('DocumentsSidebar', () => {
	function baseProps(overrides: Record<string, any> = {}) {
		return {
			projectIDs: ['smith'],
			projectsWithDocs: {
				smith: [{ id: 'doc-1', title: 'Root Doc', metadata: {} }]
			},
			selectedDocId: 'doc-1',
			showAll: false,
			searchQuery: '',
			tasksEnabled: true,
			onShowAllChange: vi.fn(),
			onSearchQueryChange: vi.fn(),
			onSelectDocument: vi.fn(),
			onBuildDocument: vi.fn(),
			onCreateTaskFromDocument: vi.fn(),
			onArchiveDocument: vi.fn(),
			onDeleteDocument: vi.fn(),
			...overrides
		};
	}

	it('keeps search visible and closes options menu on outside click', async () => {
		render(DocumentsSidebar, baseProps());

		expect(screen.getByPlaceholderText('Search documents...')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Document tree options' }));
		expect(screen.getByText('Expand all')).toBeTruthy();

		await fireEvent.mouseDown(document.body);
		expect(screen.queryByText('Expand all')).toBeNull();
	});

	it('shows Archived label for blank project group id', async () => {
		render(
			DocumentsSidebar,
			baseProps({
				projectIDs: [''],
				projectsWithDocs: {
					'': [{ id: 'doc-archived', title: 'Archived Doc', metadata: { status: 'archived' } }]
				},
				selectedDocId: 'doc-archived',
				showAll: true
			})
		);

		expect(screen.getByText('Archived')).toBeTruthy();
	});

	it('routes sidebar action menu delete through callback', async () => {
		const onDeleteDocument = vi.fn();
			render(
				DocumentsSidebar,
				baseProps({
					onDeleteDocument
				})
			);

		await fireEvent.click(screen.getByRole('button', { name: 'smith' }));

		const actionTrigger = screen.getByRole('button', { name: 'Document actions' });
		await fireEvent.click(actionTrigger);

		const deleteBtn = screen.getByRole('button', { name: 'Delete' });
		await fireEvent.click(deleteBtn);

		expect(onDeleteDocument).toHaveBeenCalledTimes(1);
		expect(onDeleteDocument.mock.calls[0][0]).toEqual(expect.objectContaining({ id: 'doc-1' }));
	});
});
