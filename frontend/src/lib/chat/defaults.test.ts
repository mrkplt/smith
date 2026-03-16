import { describe, expect, it } from 'vitest';

import {
	deriveProviderProfileID,
	loadChatSettings,
	normalizeThinkingLevel,
	resolveDefaultModel,
	resolveProviderType,
	type ChatProviderProfile,
	type StorageLike
} from '$lib/chat/defaults';

describe('chat defaults helpers', () => {
	const profiles: ChatProviderProfile[] = [
		{ id: 'codex-default', provider_type: 'codex', default_model: 'gpt-5-codex' },
		{ id: 'openai-work', provider_type: 'openai', default_model: 'gpt-5.4' },
		{ id: 'anthropic-main', provider_type: 'anthropic', default_model: 'claude-sonnet-4-5' }
	];

	it('normalizes thinking level', () => {
		expect(normalizeThinkingLevel('quick')).toBe('quick');
		expect(normalizeThinkingLevel('balanced')).toBe('balanced');
		expect(normalizeThinkingLevel('deep')).toBe('deep');
		expect(normalizeThinkingLevel('invalid')).toBe('balanced');
		expect(normalizeThinkingLevel(null)).toBe('balanced');
	});

	it('derives provider profile from stored value and legacy fallback', () => {
		expect(deriveProviderProfileID(profiles, 'openai-work', '')).toBe('openai-work');
		expect(deriveProviderProfileID(profiles, '', 'anthropic')).toBe('anthropic-main');
		expect(deriveProviderProfileID(profiles, '', '')).toBe('codex-default');
	});

	it('resolves provider type and default model', () => {
		expect(resolveProviderType(profiles, 'openai-work', '')).toBe('openai');
		expect(resolveProviderType(profiles, '', 'google')).toBe('google');
		expect(resolveDefaultModel(profiles, 'openai-work', '')).toBe('gpt-5.4');
		expect(resolveDefaultModel(profiles, 'openai-work', 'gpt-5.5-preview')).toBe('gpt-5.5-preview');
	});

	it('loads chat settings snapshot from storage', () => {
		const map = new Map<string, string>([
			['smith.chat.provider', 'openai'],
			['smith.chat.thinkingLevel', 'deep'],
			['smith.chat.providerApiKey', ' sk-test '],
			['smith.chat.preferFullScreen', 'true']
		]);
		const storage: StorageLike = {
			getItem(key: string) {
				return map.has(key) ? map.get(key)! : null;
			},
			setItem() {
				// no-op
			},
			removeItem() {
				// no-op
			}
		};

		const settings = loadChatSettings(storage, profiles);
		expect(settings.providerProfileID).toBe('openai-work');
		expect(settings.provider).toBe('openai');
		expect(settings.defaultModel).toBe('gpt-5.4');
		expect(settings.thinkingLevel).toBe('deep');
		expect(settings.providerApiKey).toBe('sk-test');
		expect(settings.preferFullScreen).toBe(true);
	});
});
