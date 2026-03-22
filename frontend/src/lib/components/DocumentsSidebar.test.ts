import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';

import DocumentsSidebar from './DocumentsSidebar.svelte';

describe('DocumentsSidebar', () => {
	it('keeps search visible and closes options menu on outside click', async () => {
		render(DocumentsSidebar, {
			projectIDs: ['smith'],
			projectsWithDocs: {
				smith: [{ id: 'doc-1', title: 'Root Doc', metadata: {} }]
			},
			selectedDocId: 'doc-1',
			showAll: false,
			searchQuery: '',
			onShowAllChange: vi.fn(),
			onSearchQueryChange: vi.fn(),
			onSelectDocument: vi.fn()
		});

		expect(screen.getByPlaceholderText('Search documents...')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Document tree options' }));
		expect(screen.getByText('Expand all')).toBeTruthy();

		await fireEvent.mouseDown(document.body);
		expect(screen.queryByText('Expand all')).toBeNull();
	});
});
