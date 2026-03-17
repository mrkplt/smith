export type ThinkingLevel = 'quick' | 'balanced' | 'deep';

export interface ChatProviderProfile {
	id: string;
	provider_type?: string;
	default_model?: string;
}

export interface ChatSettingsSnapshot {
	providerProfileID: string;
	provider: string;
	defaultModel: string;
	thinkingLevel: ThinkingLevel;
	providerApiKey: string;
	preferFullScreen: boolean;
}

export interface StorageLike {
	getItem(key: string): string | null;
	setItem(key: string, value: string): void;
	removeItem(key: string): void;
}

/** Normalizes persisted thinking level values to the supported enum. */
export function normalizeThinkingLevel(value: string | null | undefined): ThinkingLevel {
	if (value === 'quick' || value === 'balanced' || value === 'deep') {
		return value;
	}
	return 'balanced';
}

/** Finds a provider profile by ID when present in the loaded profile list. */
export function findProviderProfile(
	profiles: ChatProviderProfile[],
	providerProfileID: string
): ChatProviderProfile | null {
	const target = providerProfileID.trim();
	if (target === '') {
		return null;
	}
	return profiles.find((profile) => profile.id === target) || null;
}

/**
 * Resolves the active provider profile ID from stored settings with legacy fallback.
 */
export function deriveProviderProfileID(
	profiles: ChatProviderProfile[],
	storedProfileID: string,
	legacyProvider: string
): string {
	const normalizedStored = storedProfileID.trim();
	if (normalizedStored !== '' && findProviderProfile(profiles, normalizedStored)) {
		return normalizedStored;
	}
	const inferredFromLegacy = inferProviderProfileFromLegacyProvider(profiles, legacyProvider);
	if (inferredFromLegacy !== '') {
		return inferredFromLegacy;
	}
	if (findProviderProfile(profiles, 'codex-default')) {
		return 'codex-default';
	}
	if (profiles.length > 0) {
		return profiles[0].id;
	}
	return '';
}

/** Resolves provider type from selected profile, falling back to legacy provider value. */
export function resolveProviderType(
	profiles: ChatProviderProfile[],
	providerProfileID: string,
	fallbackProvider: string
): string {
	const profile = findProviderProfile(profiles, providerProfileID);
	const fromProfile = String(profile?.provider_type || '').trim().toLowerCase();
	if (fromProfile !== '') {
		return fromProfile;
	}
	return fallbackProvider.trim().toLowerCase();
}

/** Resolves model from explicit setting or selected provider profile defaults. */
export function resolveDefaultModel(
	profiles: ChatProviderProfile[],
	providerProfileID: string,
	storedDefaultModel: string
): string {
	const explicit = storedDefaultModel.trim();
	if (explicit !== '') {
		return explicit;
	}
	const profile = findProviderProfile(profiles, providerProfileID);
	return String(profile?.default_model || '').trim();
}

/** Loads chat settings snapshot from local storage and provider profile metadata. */
export function loadChatSettings(storage: StorageLike, profiles: ChatProviderProfile[]): ChatSettingsSnapshot {
	const legacyProvider = storage.getItem('smith.chat.provider') || '';
	const providerProfileID = deriveProviderProfileID(
		profiles,
		storage.getItem('smith.chat.providerProfileID') || '',
		legacyProvider
	);
	const provider = resolveProviderType(profiles, providerProfileID, legacyProvider);
	const defaultModel = resolveDefaultModel(
		profiles,
		providerProfileID,
		storage.getItem('smith.chat.defaultModel') || ''
	);

	return {
		providerProfileID,
		provider,
		defaultModel,
		thinkingLevel: normalizeThinkingLevel(storage.getItem('smith.chat.thinkingLevel')),
		providerApiKey: (storage.getItem('smith.chat.providerApiKey') || '').trim(),
		preferFullScreen: storage.getItem('smith.chat.preferFullScreen') === 'true'
	};
}

function inferProviderProfileFromLegacyProvider(
	profiles: ChatProviderProfile[],
	legacyProvider: string
): string {
	const normalizedProvider = legacyProvider.trim().toLowerCase();
	if (normalizedProvider === '') {
		return '';
	}
	const match = profiles.find(
		(profile) => String(profile.provider_type || '').trim().toLowerCase() === normalizedProvider
	);
	return match?.id || '';
}
