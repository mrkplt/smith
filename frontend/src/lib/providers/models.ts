import { fetchJSON } from '$lib/api';

const modelCache = new Map<string, string[]>();

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

function uniqueModels(values: string[]): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const value of values) {
		const model = String(value || '').trim();
		if (model === '' || seen.has(model)) {
			continue;
		}
		seen.add(model);
		out.push(model);
	}
	return out;
}

/**
 * Returns the static model fallback list for a provider type.
 */
export function staticModelsForProviderType(providerType: string): string[] {
	switch (canonicalProviderType(providerType)) {
	case 'claude':
		return ['claude-sonnet-4-5', 'claude-haiku-4-5'];
	case 'gemini':
		return ['gemini-2.5-pro', 'gemini-2.5-flash'];
	case 'codex':
	default:
		return ['gpt-4.1', 'gpt-5-codex', 'gpt-5-codex-mini'];
	}
}

/**
 * Loads models for a provider profile and falls back to static defaults.
 */
export async function loadProviderModels(providerProfileID: string, providerTypeHint = '', allowDynamicLookup = true): Promise<string[]> {
	const profileID = String(providerProfileID || '').trim();
	if (profileID === '') {
		return staticModelsForProviderType(providerTypeHint);
	}
	if (!allowDynamicLookup) {
		return staticModelsForProviderType(providerTypeHint);
	}
	if (modelCache.has(profileID)) {
		return modelCache.get(profileID) || [];
	}

	try {
		const response = await fetchJSON(`/v1/providers/${encodeURIComponent(profileID)}/models`);
		const apiModels = Array.isArray(response?.models)
			? response.models.map((item: any) => String(item?.id || '').trim())
			: [];
		const providerType = String(response?.provider_type || providerTypeHint || '').trim();
		const defaultModel = String(response?.default_model || '').trim();
		const models = uniqueModels([...apiModels, defaultModel, ...staticModelsForProviderType(providerType)]);
		modelCache.set(profileID, models);
		return models;
	} catch {
		const fallback = staticModelsForProviderType(providerTypeHint);
		modelCache.set(profileID, fallback);
		return fallback;
	}
}

/**
 * Ensures a selected model is present in a model list.
 */
export function includeSelectedModel(models: string[], selectedModel: string): string[] {
	const selected = String(selectedModel || '').trim();
	if (selected === '') {
		return uniqueModels(models);
	}
	return uniqueModels([...models, selected]);
}

/**
 * Clears model cache for one provider or all providers.
 */
export function clearProviderModelsCache(providerProfileID = ''): void {
	const profileID = String(providerProfileID || '').trim();
	if (profileID === '') {
		modelCache.clear();
		return;
	}
	modelCache.delete(profileID);
}
