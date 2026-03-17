import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';

Object.defineProperty(window, 'matchMedia', {
	writable: true,
	value: vi.fn().mockImplementation((query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	}))
});

vi.mock('$lib/api', () => ({
	getJSON: vi.fn(),
	postJSON: vi.fn(),
	requestJSON: vi.fn(),
	deleteJSON: vi.fn()
}));

vi.mock('$lib/stores', () => ({
	pushToast: vi.fn()
}));

import * as api from '$lib/api';
import * as stores from '$lib/stores';
import ProviderEditorDrawer from './ProviderEditorDrawer.svelte';

describe('ProviderEditorDrawer', () => {
	const catalog = [
		{ id: 'codex', display_name: 'Codex', required_config_fields: ['id', 'provider_type'] },
		{ id: 'claude', display_name: 'Claude', required_config_fields: ['id', 'provider_type', 'secret_ref'] },
		{ id: 'gemini', display_name: 'Gemini', required_config_fields: ['id', 'provider_type', 'secret_ref'] }
	];

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.getJSON).mockImplementation(async (path: string) => {
			if (path === '/v1/providers/catalog') {
				return catalog;
			}
			if (path === '/v1/auth/codex/credential') {
				return { connected: false };
			}
			return {};
		});
		vi.mocked(api.postJSON).mockResolvedValue({});
		vi.mocked(api.requestJSON).mockResolvedValue({});
	});

	it('renders secret reference suggestions', () => {
		const { container } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: [
				{ id: 'openai-key', name: 'OpenAI key' },
				{ id: 'anthropic-key', name: 'Anthropic key' }
			]
		});

		const options = Array.from(container.querySelectorAll('#provider-secret-options option')).map((node) => node.getAttribute('value'));
		expect(options).toContain('openai-key');
		expect(options).toContain('anthropic-key');

		cleanup();
	});

	it('restricts provider type choices to supported catalog entries', async () => {
		const { getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		const select = getByTestId('provider-type') as HTMLSelectElement;
		await waitFor(() => expect(select.options.length).toBe(3));
		expect(Array.from(select.options).map((option) => option.value)).toEqual(['codex', 'claude', 'gemini']);

		cleanup();
	});

	it('submits secret_ref in provider profile payload', async () => {
		const onClose = vi.fn();
		const onSaved = vi.fn();
		const { container, getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose,
			onSaved,
			provider: null,
			secretOptions: [{ id: 'openai-key', name: 'OpenAI key' }]
		});

		await fireEvent.input(getByTestId('provider-profile-id'), { target: { value: 'openai-work' } });
		await fireEvent.input(getByTestId('provider-display-name'), { target: { value: 'OpenAI Work' } });
		await fireEvent.input(getByTestId('provider-secret-ref'), { target: { value: 'openai-key' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => expect(api.postJSON).toHaveBeenCalled());
		const [path, payload] = vi.mocked(api.postJSON).mock.calls[0] || [];
		expect(path).toBe('/v1/providers');
		expect(payload).toMatchObject({
			id: 'openai-work',
			name: 'OpenAI Work',
			provider_type: 'codex',
			secret_ref: 'openai-key'
		});
		expect(stores.pushToast).toHaveBeenCalledWith('Provider profile created successfully', 'ok');
		expect(onSaved).toHaveBeenCalled();
		expect(onClose).toHaveBeenCalled();

		cleanup();
	});

	it('requires secret reference for providers that declare secret_ref', async () => {
		const { container } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: {
				id: 'claude-team',
				name: 'Claude Team',
				provider_type: 'claude'
			},
			secretOptions: []
		});

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => {
			expect(stores.pushToast).toHaveBeenCalledWith('Secret reference is required for Claude providers', 'err');
		});
		expect(api.requestJSON).not.toHaveBeenCalled();
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/providers', expect.anything());

		cleanup();
	});

	it('rejects invalid codex api key format before save', async () => {
		const { container, getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		await fireEvent.input(getByTestId('provider-profile-id'), { target: { value: 'codex-team' } });
		const apiKeyField = container.querySelector('input[placeholder="sk-..."]') as HTMLInputElement;
		expect(apiKeyField).toBeTruthy();
		await fireEvent.input(apiKeyField, { target: { value: 'invalid-key' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => {
			expect(stores.pushToast).toHaveBeenCalledWith('Codex API key must start with sk-', 'err');
		});
		expect(api.requestJSON).not.toHaveBeenCalled();
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/auth/codex/connect/api-key', expect.anything());

		cleanup();
	});

	it('revokes codex credential from drawer action', async () => {
		vi.mocked(api.getJSON).mockImplementation(async (path: string) => {
			if (path === '/v1/providers/catalog') {
				return catalog;
			}
			if (path === '/v1/auth/codex/credential') {
				return {
					connected: true,
					account_id: 'default',
					api_key_masked: 'sk-***1234',
					last_refresh_at: '2026-03-16T00:00:00Z'
				};
			}
			return {};
		});

		const { getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		const revokeButton = await waitFor(() => getByTestId('provider-codex-revoke'));
		await fireEvent.click(revokeButton);

		await waitFor(() => {
			expect(api.postJSON).toHaveBeenCalledWith('/v1/auth/codex/disconnect', { actor: 'operator' });
		});
		expect(stores.pushToast).toHaveBeenCalledWith('Codex credential revoked', 'ok');

		cleanup();
	});
});
