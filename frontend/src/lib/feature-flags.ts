type RuntimeConfig = {
  featureTasksEnabled?: boolean | string;
  featureCapabilityEnabled?: boolean | string;
  featureProviderClaudeEnabled?: boolean | string;
  featureProviderGeminiEnabled?: boolean | string;
};

/** Parses a runtime boolean-like value (boolean or string) into strict boolean/null. */
export function parseBoolean(value: boolean | string | undefined): boolean | null {
  if (typeof value === 'boolean') {
    return value;
  }
  if (typeof value !== 'string') {
    return null;
  }

  const normalized = value.trim().toLowerCase();
  if (normalized === 'true' || normalized === '1' || normalized === 'yes') {
    return true;
  }
  if (normalized === 'false' || normalized === '0' || normalized === 'no') {
    return false;
  }
  return null;
}

/** Returns the browser-injected runtime config object used by console feature gates. */
export function getRuntimeConfig(): RuntimeConfig {
  if (typeof window === 'undefined') {
    return {};
  }
  return ((window as any).__SMITH_CONFIG__ || {}) as RuntimeConfig;
}

function parseFlag(value: boolean | string | undefined, fallback: boolean): boolean {
  const parsed = parseBoolean(value);
  return parsed === null ? fallback : parsed;
}

/** Returns whether the Tasks runtime surface is enabled. */
export function isTasksEnabled(config = getRuntimeConfig()): boolean {
  return parseFlag(config.featureTasksEnabled, false);
}

/** Returns whether the Feature Capability runtime surface is enabled. */
export function isFeatureCapabilityEnabled(config = getRuntimeConfig()): boolean {
  return parseFlag(config.featureCapabilityEnabled, false);
}

/** Returns whether a shell nav/runtime feature is visible in current config. */
export function isFeatureVisible(featureID: string, config = getRuntimeConfig()): boolean {
  if (featureID === 'tasks') {
    return isTasksEnabled(config);
  }
  if (featureID === 'feature-capability') {
    return isFeatureCapabilityEnabled(config);
  }
  return true;
}

/** Returns whether a provider type is enabled in the current runtime config. */
export function isProviderTypeEnabled(providerType: string, config = getRuntimeConfig()): boolean {
  const normalized = String(providerType || '').trim().toLowerCase();
  if (normalized === 'codex' || normalized === 'openai' || normalized === '') {
    return true;
  }
  if (normalized === 'claude' || normalized === 'anthropic') {
    return parseFlag(config.featureProviderClaudeEnabled, false);
  }
  if (normalized === 'gemini' || normalized === 'google') {
    return parseFlag(config.featureProviderGeminiEnabled, false);
  }
  return false;
}
