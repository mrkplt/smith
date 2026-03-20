<script lang="ts">
	import { getJSON, postJSON, requestJSON, deleteJSON } from '$lib/api';
	import { isProviderTypeEnabled } from '$lib/feature-flags';
	import { includeSelectedModel, loadProviderModels, staticModelsForProviderType } from '$lib/providers/models';
	import { pushToast } from '$lib/stores';
	import { Drawer, Button, Input, Label, Helper } from 'flowbite-svelte';
	import { CheckOutline, CloseOutline, AdjustmentsHorizontalOutline, TrashBinOutline } from 'flowbite-svelte-icons';
	import { sineIn } from 'svelte/easing';

	interface Props {
		open: boolean;
		onClose: () => void;
		onSaved?: () => void;
		provider?: any;
		secretOptions?: any[];
	}

	let { open = $bindable(), onClose, onSaved = () => {}, provider = null, secretOptions = [] }: Props = $props();

	let id = $state('');
	let name = $state('');
	let providerType = $state('codex');
	let endpoint = $state('');
	let defaultModel = $state('');
	let capabilities = $state('chat,tools,loops');
	let secretRef = $state('');
	let apiKey = $state('');
	let accountId = $state('');
	let providerModelOptions = $state<string[]>([]);
	let providerModelOptionsBusy = $state(false);
	let codexCredential = $state<any>({ connected: false });
	let codexCredentialBusy = $state(false);
	let busy = $state(false);
	const fallbackProviderCatalog = [
		{ id: 'codex', display_name: 'Codex', required_config_fields: ['id', 'provider_type'] },
		{ id: 'claude', display_name: 'Claude', required_config_fields: ['id', 'provider_type', 'secret_ref'] },
		{ id: 'gemini', display_name: 'Gemini', required_config_fields: ['id', 'provider_type', 'secret_ref'] }
	];

	function filterCatalogByProviderFlags(catalog: any[]): any[] {
		if (!Array.isArray(catalog)) {
			return [];
		}
		return catalog.filter((entry) => {
			const providerID = String(entry?.provider_type || entry?.id || '').trim().toLowerCase();
			return isProviderTypeEnabled(providerID);
		});
	}

	let providerCatalog = $state<any[]>(filterCatalogByProviderFlags(fallbackProviderCatalog));

	let isHidden = $state(true);
	$effect(() => {
		isHidden = !open;
	});
	$effect(() => {
		if (open && isHidden) {
			onClose();
		}
	});

	let transitionParams = {
		x: 450,
		duration: 300,
		easing: sineIn
	};

	const isEditing = $derived(!!provider?.id);
	const isProtected = $derived(String(id).trim() === 'codex-default');
	const normalizedProviderType = $derived(canonicalProviderType(providerType));
	const selectedCatalogEntry = $derived(providerCatalog.find((entry) => entry.id === providerType) || null);
	const requiredFieldsSummary = $derived(
		selectedCatalogEntry && Array.isArray(selectedCatalogEntry.required_config_fields)
			? selectedCatalogEntry.required_config_fields.join(', ')
			: ''
	);
	const selectedProviderLabel = $derived(String(selectedCatalogEntry?.display_name || providerType || 'provider'));
	const requiresSecretRef = $derived(
		!!(selectedCatalogEntry && Array.isArray(selectedCatalogEntry.required_config_fields) && selectedCatalogEntry.required_config_fields.includes('secret_ref'))
	);
	const availableProviderModelOptions = $derived(includeSelectedModel(providerModelOptions, defaultModel));

	function canonicalProviderType(raw: string): string {
		const normalized = String(raw || '').trim().toLowerCase();
		if (normalized === '' || normalized === 'codex' || normalized === 'openai') {
			return 'codex';
		}
		if (normalized === 'claude' || normalized === 'anthropic') {
			return 'claude';
		}
		if (normalized === 'gemini' || normalized === 'google') {
			return 'gemini';
		}
		return normalized;
	}

	async function loadProviderCatalog() {
		try {
			const response = await getJSON('/v1/providers/catalog');
			const filteredResponse = filterCatalogByProviderFlags(response);
			if (filteredResponse.length > 0) {
				providerCatalog = filteredResponse;
				return;
			}
			providerCatalog = filterCatalogByProviderFlags(fallbackProviderCatalog);
		} catch {
			// Keep built-in fallback catalog for local/offline usage.
			providerCatalog = filterCatalogByProviderFlags(fallbackProviderCatalog);
		}
	}

	async function loadCodexCredential() {
		if (canonicalProviderType(providerType) !== 'codex') {
			codexCredential = { connected: false };
			return;
		}
		codexCredentialBusy = true;
		try {
			const response = await getJSON('/v1/auth/codex/credential');
			codexCredential = response || { connected: false };
		} catch {
			codexCredential = { connected: false };
		} finally {
			codexCredentialBusy = false;
		}
	}

	async function loadProviderModelOptions() {
		const canonicalType = canonicalProviderType(providerType);
		providerModelOptionsBusy = true;
		try {
			if (isEditing && id.trim() !== '') {
				providerModelOptions = await loadProviderModels(id.trim(), canonicalType);
				return;
			}
			providerModelOptions = staticModelsForProviderType(canonicalType);
		} finally {
			providerModelOptionsBusy = false;
		}
	}

	async function disconnectCodexCredential() {
		codexCredentialBusy = true;
		try {
			await postJSON('/v1/auth/codex/disconnect', { actor: 'operator' });
			await loadCodexCredential();
			pushToast('Codex credential revoked', 'ok');
		} catch (err: any) {
			pushToast(err?.message || 'Failed to revoke Codex credential', 'err');
		} finally {
			codexCredentialBusy = false;
		}
	}

	$effect(() => {
		if (!open) {
			return;
		}
		void loadProviderCatalog();
		if (provider) {
			id = provider.id || '';
			name = provider.name || '';
			providerType = canonicalProviderType(provider.provider_type || 'codex');
			endpoint = provider.endpoint || '';
			defaultModel = provider.default_model || '';
			capabilities = Array.isArray(provider.capabilities)
				? provider.capabilities.join(',')
				: 'chat,tools,loops';
			secretRef = provider.secret_ref || '';
		} else {
			id = '';
			name = '';
			providerType = 'codex';
			endpoint = '';
			defaultModel = '';
			capabilities = 'chat,tools,loops';
			secretRef = '';
		}
		apiKey = '';
		accountId = '';
		void loadProviderModelOptions();
		void loadCodexCredential();
	});

	$effect(() => {
		if (!open) {
			return;
		}
		if (providerType.trim() === '') {
			return;
		}
		void loadProviderModelOptions();
		void loadCodexCredential();
	});

	$effect(() => {
		if (!open || provider) {
			return;
		}
		if (!providerCatalog.some((entry) => entry.id === providerType) && providerCatalog.length > 0) {
			providerType = String(providerCatalog[0].id || 'codex');
		}
	});

	async function saveProvider() {
		if (id.trim() === '') {
			pushToast('Provider profile id is required', 'err');
			return;
		}
		if (requiresSecretRef && secretRef.trim() === '') {
			pushToast(`Secret reference is required for ${selectedProviderLabel} providers`, 'err');
			return;
		}
		if (canonicalProviderType(providerType) === 'codex' && apiKey.trim() !== '' && !apiKey.trim().startsWith('sk-')) {
			pushToast('Codex API key must start with sk-', 'err');
			return;
		}
		busy = true;
		try {
			const canonicalType = canonicalProviderType(providerType);
			const payload = {
				id: id.trim(),
				name: name.trim(),
				provider_type: canonicalType,
				endpoint: endpoint.trim(),
				default_model: defaultModel.trim(),
				capabilities: capabilities
					.split(',')
					.map((value) => value.trim())
					.filter((value) => value !== ''),
				secret_ref: secretRef.trim()
			};
			if (isEditing) {
				await requestJSON(`/v1/providers/${payload.id}`, 'PUT', payload);
			} else {
				await postJSON('/v1/providers', payload);
			}

			if (canonicalType === 'codex' && apiKey.trim() !== '') {
				await postJSON('/v1/auth/codex/connect/api-key', {
					actor: 'operator',
					api_key: apiKey.trim(),
					account_id: accountId.trim() || 'default'
				});
			}

			pushToast(`Provider profile ${isEditing ? 'updated' : 'created'} successfully`, 'ok');
			onSaved();
			onClose();
		} catch (err: any) {
			pushToast(err?.message || 'Failed to save provider profile', 'err');
		} finally {
			busy = false;
		}
	}

	async function deleteProviderProfile() {
		if (isProtected) {
			pushToast('Default provider profile cannot be deleted', 'err');
			return;
		}
		if (!confirm(`Delete provider profile ${id}?`)) {
			return;
		}
		busy = true;
		try {
			await deleteJSON(`/v1/providers/${id}`);
			pushToast('Provider profile deleted', 'ok');
			onSaved();
			onClose();
		} catch (err: any) {
			pushToast(err?.message || 'Failed to delete provider profile', 'err');
		} finally {
			busy = false;
		}
	}
</script>

<Drawer
	placement="right"
	bind:hidden={isHidden}
	outsideclose={true}
	id="provider-editor-drawer"
	width="default"
	class="fixed top-0 right-0 bg-black border-l border-gray-800 z-50 overflow-y-auto h-full m-0 shadow-2xl p-0 w-[450px]"
>
	<div class="flex flex-col h-full relative">
		<div class="px-8 py-10 bg-slate-900/20 border-b border-gray-900 flex items-center justify-between">
			<div class="flex items-center gap-4">
				<div class="w-12 h-12 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
					<AdjustmentsHorizontalOutline size="lg" />
				</div>
				<div>
					<h2 class="text-2xl font-bold text-white uppercase tracking-tighter">{isEditing ? 'Edit Provider' : 'New Provider'}</h2>
					<p class="text-[10px] font-bold text-gray-500 uppercase tracking-[0.2em] mt-1">Profile Configuration</p>
				</div>
			</div>
			<button class="text-white hover:text-[#86BC25] transition-colors p-2" onclick={onClose} aria-label="Close Drawer">
				<CloseOutline size="md" />
			</button>
		</div>

		<form class="flex-1 p-8 space-y-8" onsubmit={(event) => { event.preventDefault(); void saveProvider(); }}>
			<div class="space-y-5">
				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Profile ID</Label>
					<Input
						data-testid="provider-profile-id"
						type="text"
						value={id}
						oninput={(event) => id = (event.currentTarget as HTMLInputElement).value}
						disabled={busy || isEditing}
						class="bg-black border-gray-800 text-white font-mono rounded-none"
					/>
					<Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">Stable reference used by projects and automation.</Helper>
				</div>

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Display Name</Label>
					<Input
						data-testid="provider-display-name"
						type="text"
						value={name}
						oninput={(event) => name = (event.currentTarget as HTMLInputElement).value}
						disabled={busy}
						class="bg-black border-gray-800 text-white rounded-none"
					/>
				</div>

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Provider Type</Label>
					<select
						data-testid="provider-type"
						class="w-full bg-black border border-gray-800 text-white text-sm rounded-none px-3 py-2"
						value={providerType}
						oninput={(event) => providerType = (event.currentTarget as HTMLSelectElement).value}
						disabled={busy}
					>
						{#each providerCatalog as entry}
							<option value={entry.id}>{entry.display_name || entry.id}</option>
						{/each}
					</select>
					{#if requiredFieldsSummary !== ''}
						<Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">Required: {requiredFieldsSummary}</Helper>
					{/if}
				</div>

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Default Model</Label>
					<select
						class="w-full bg-black border border-gray-800 text-white text-sm rounded-none px-3 py-2"
						value={defaultModel}
						oninput={(event) => defaultModel = (event.currentTarget as HTMLSelectElement).value}
						disabled={busy}
					>
						<option value="">Use provider default</option>
						{#if providerModelOptionsBusy}
							<option value={defaultModel} disabled>{defaultModel !== '' ? defaultModel : 'Loading models...'}</option>
						{/if}
						{#each availableProviderModelOptions as model}
							<option value={model}>{model}</option>
						{/each}
					</select>
				</div>

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Endpoint (Optional)</Label>
					<Input
						type="text"
						value={endpoint}
						oninput={(event) => endpoint = (event.currentTarget as HTMLInputElement).value}
						disabled={busy}
						placeholder="https://api.openai.com/v1"
						class="bg-black border-gray-800 text-white rounded-none"
					/>
				</div>

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Capabilities</Label>
					<Input
						type="text"
						value={capabilities}
						oninput={(event) => capabilities = (event.currentTarget as HTMLInputElement).value}
						disabled={busy}
						placeholder="chat,tools,loops"
						class="bg-black border-gray-800 text-white rounded-none"
					/>
				</div>

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Secret Reference (Optional)</Label>
					<Input
						data-testid="provider-secret-ref"
						list="provider-secret-options"
						type="text"
						value={secretRef}
						oninput={(event) => secretRef = (event.currentTarget as HTMLInputElement).value}
						disabled={busy}
						placeholder="openai-key"
						class="bg-black border-gray-800 text-white rounded-none"
					/>
					<datalist id="provider-secret-options">
						{#each secretOptions as secretRecord}
							<option value={secretRecord.id}>{secretRecord.name || secretRecord.id}</option>
						{/each}
					</datalist>
				</div>

				{#if normalizedProviderType === 'codex'}
					<div class="space-y-4 pt-4 border-t border-gray-900">
						<div class="border border-gray-800 bg-slate-900/30 p-3 space-y-2">
							<div class="text-[10px] uppercase tracking-[0.2em] font-bold text-gray-500">Credential Status</div>
							<div class="text-xs text-gray-200">{codexCredential.connected ? 'Connected' : 'Not connected'}</div>
							{#if codexCredential.connected}
								<div class="space-y-1 text-[11px] text-gray-400">
									{#if codexCredential.api_key_masked}
										<div>Key: <span class="font-mono">{codexCredential.api_key_masked}</span></div>
									{/if}
									{#if codexCredential.account_id}
										<div>Account: <span class="font-mono">{codexCredential.account_id}</span></div>
									{/if}
									{#if codexCredential.last_refresh_at}
										<div>Last Refresh: <span class="font-mono">{codexCredential.last_refresh_at}</span></div>
									{/if}
								</div>
							{/if}
							<div class="pt-1">
								<Button
									size="xs"
									color="alternative"
									class="rounded-none border-gray-700 bg-slate-900 text-gray-300"
									data-testid="provider-codex-revoke"
									onclick={disconnectCodexCredential}
									disabled={codexCredentialBusy || !codexCredential.connected}
								>
									Revoke Credential
								</Button>
							</div>
						</div>

						<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Codex API Key (Optional)</Label>
						<Input
							type="password"
							value={apiKey}
							oninput={(event) => apiKey = (event.currentTarget as HTMLInputElement).value}
							disabled={busy}
							placeholder="sk-..."
							class="bg-black border-gray-800 text-white rounded-none"
						/>
						<Input
							type="text"
							value={accountId}
							oninput={(event) => accountId = (event.currentTarget as HTMLInputElement).value}
							disabled={busy}
							placeholder="account id (optional)"
							class="bg-black border-gray-800 text-white rounded-none"
						/>
						<Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">Leave blank to keep current credential. Set a new key to rotate.</Helper>
					</div>
				{/if}
			</div>

			<div class="pt-10 pb-20 border-t border-gray-900 flex justify-between gap-4 mt-auto">
				{#if isEditing && !isProtected}
					<Button color="red" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest px-6" onclick={deleteProviderProfile} disabled={busy}>
						<TrashBinOutline size="xs" class="mr-2" />
						Delete
					</Button>
				{/if}
				<div class="flex gap-3 ml-auto">
					<Button color="alternative" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest border-gray-700 bg-slate-900 text-gray-300 hover:bg-slate-700 px-6" onclick={onClose} disabled={busy}>Cancel</Button>
					<Button color="alternative" data-testid="provider-save" class="bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest rounded-none px-8 py-2 hover:bg-[#a1e02c]" type="submit" disabled={busy}>
						<CheckOutline size="xs" class="mr-2" />
						{isEditing ? 'Update Profile' : 'Create Profile'}
					</Button>
				</div>
			</div>
		</form>
	</div>
</Drawer>

<style>
	:global(#provider-editor-drawer) {
		background-color: #000000 !important;
		left: auto !important;
		right: 0 !important;
	}
	:global(#provider-editor-drawer button[aria-label="Close"]) {
		display: none !important;
	}
	:global(#provider-editor-drawer .relative > button[aria-label="Close Drawer"]) {
		display: flex !important;
	}
</style>
