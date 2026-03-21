import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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
		{ id: 'codex', display_name: 'Codex', required_config_fields: ['id', 'provider_type', 'secret_ref'] },
		{ id: 'claude', display_name: 'Claude', required_config_fields: ['id', 'provider_type', 'secret_ref'] },
		{ id: 'gemini', display_name: 'Gemini', required_config_fields: ['id', 'provider_type', 'secret_ref'] }
	];

	beforeEach(() => {
		vi.clearAllMocks();
		(window as any).__SMITH_CONFIG__ = {};
		vi.mocked(api.getJSON).mockImplementation(async (path: string) => {
			if (path === '/v1/providers/catalog') {
				return catalog;
			}
			return {};
		});
		vi.mocked(api.postJSON).mockResolvedValue({});
		vi.mocked(api.requestJSON).mockResolvedValue({});
	});

	afterEach(() => {
		cleanup();
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
		(window as any).__SMITH_CONFIG__ = {
			featureProviderClaudeEnabled: false,
			featureProviderGeminiEnabled: false
		};
		const { getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		const select = getByTestId('provider-type') as HTMLSelectElement;
		await waitFor(() => expect(select.options.length).toBe(1));
		expect(Array.from(select.options).map((option) => option.value)).toEqual(['codex']);

		cleanup();
	});

	it('shows claude and gemini types when provider flags are enabled', async () => {
		(window as any).__SMITH_CONFIG__ = {
			featureProviderClaudeEnabled: true,
			featureProviderGeminiEnabled: true
		};
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

	it('upserts secret from API key and submits provider with secret_ref', async () => {
		const onClose = vi.fn();
		const onSaved = vi.fn();
		const { container, getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose,
			onSaved,
			provider: null,
			secretOptions: []
		});

		await fireEvent.input(getByTestId('provider-profile-id'), { target: { value: 'openai-work' } });
		await fireEvent.input(getByTestId('provider-display-name'), { target: { value: 'OpenAI Work' } });
		await fireEvent.input(getByTestId('provider-credential-id'), { target: { value: 'openai-key' } });
		await fireEvent.input(getByTestId('provider-api-key'), { target: { value: 'sk-test-123' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => expect(api.requestJSON).toHaveBeenCalled());
		expect(api.requestJSON).toHaveBeenCalledWith(
			'/v1/secrets/openai-key',
			'PUT',
			expect.objectContaining({
				id: 'openai-key',
				name: 'openai-key',
				value: 'sk-test-123'
			})
		);
		expect(api.postJSON).toHaveBeenCalledWith(
			'/v1/providers',
			expect.objectContaining({
			id: 'openai-work',
			name: 'OpenAI Work',
			provider_type: 'codex',
			secret_ref: 'openai-key'
			})
		);
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/secrets', expect.anything());
		expect(stores.pushToast).toHaveBeenCalledWith('Provider profile created successfully', 'ok');
		expect(onSaved).toHaveBeenCalled();
		expect(onClose).toHaveBeenCalled();

		cleanup();
	});

	it('creates secret when secret PUT returns not found', async () => {
		vi.mocked(api.requestJSON).mockRejectedValueOnce(new Error('not found'));
		const { container, getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		await fireEvent.input(getByTestId('provider-profile-id'), { target: { value: 'codex-team' } });
		await fireEvent.input(getByTestId('provider-credential-id'), { target: { value: 'codex-team-key' } });
		await fireEvent.input(getByTestId('provider-api-key'), { target: { value: 'sk-codex-123' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => {
			expect(api.postJSON).toHaveBeenCalledWith(
				'/v1/secrets',
				expect.objectContaining({ id: 'codex-team-key', value: 'sk-codex-123' })
			);
		});
		expect(api.postJSON).toHaveBeenCalledWith('/v1/providers', expect.anything());

		cleanup();
	});

	it('requires credential label for providers that declare secret_ref', async () => {
		(window as any).__SMITH_CONFIG__ = {
			featureProviderClaudeEnabled: true
		};
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
			expect(stores.pushToast).toHaveBeenCalledWith('Credential label is required for Claude providers', 'err');
		});
		expect(api.requestJSON).not.toHaveBeenCalled();
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/providers', expect.anything());

		cleanup();
	});

	it('requires credential label for codex providers', async () => {
		const { container, getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		await fireEvent.input(getByTestId('provider-profile-id'), { target: { value: 'codex-team' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => {
			expect(stores.pushToast).toHaveBeenCalledWith('Credential label is required for Codex providers', 'err');
		});
		expect(api.requestJSON).not.toHaveBeenCalled();
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/providers', expect.anything());

		cleanup();
	});

	it('requires API key when creating a new credential label', async () => {
		const { container, getByTestId } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: null,
			secretOptions: []
		});

		await fireEvent.input(getByTestId('provider-profile-id'), { target: { value: 'codex-team' } });
		await fireEvent.input(getByTestId('provider-credential-id'), { target: { value: 'codex-team-key' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => {
			expect(stores.pushToast).toHaveBeenCalledWith('API key is required when creating a new credential label', 'err');
		});
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/providers', expect.anything());

		cleanup();
	});

	it('treats openai provider alias as codex and keeps existing credential label without key rotation', async () => {
		const { container } = render(ProviderEditorDrawer, {
			open: true,
			onClose: vi.fn(),
			onSaved: vi.fn(),
			provider: {
				id: 'openai-work',
				name: 'OpenAI Work',
				provider_type: 'openai',
				default_model: 'gpt-4.1',
				secret_ref: 'openai-key'
			},
			secretOptions: []
		});

		const secretField = container.querySelector('[data-testid="provider-credential-id"]') as HTMLInputElement;
		expect(secretField).toBeTruthy();
		await fireEvent.input(secretField, { target: { value: 'openai-key' } });

		const form = container.querySelector('form');
		expect(form).toBeTruthy();
		await fireEvent.submit(form!);

		await waitFor(() => expect(api.requestJSON).toHaveBeenCalled());
		expect(api.requestJSON).toHaveBeenCalledWith(
			'/v1/providers/openai-work',
			'PUT',
			expect.objectContaining({ provider_type: 'codex', secret_ref: 'openai-key' })
		);
		expect(api.postJSON).not.toHaveBeenCalledWith('/v1/secrets', expect.anything());

		cleanup();
	});
});
