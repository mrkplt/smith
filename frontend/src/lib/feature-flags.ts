type RuntimeConfig = {
  featureTasksEnabled?: boolean | string;
  featureCapabilityEnabled?: boolean | string;
  featureChatEnabled?: boolean | string;
  featureIntegrationsEnabled?: boolean | string;
  featureSecretsEnabled?: boolean | string;
  featureProviderClaudeEnabled?: boolean | string;
  featureProviderGeminiEnabled?: boolean | string;
  featurePRDDiagnosticResolveEnabled?: boolean | string;
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

/** Returns whether chat surfaces are enabled in the current runtime config. */
export function isChatEnabled(config = getRuntimeConfig()): boolean {
  return parseFlag(config.featureChatEnabled, false);
}

/** Returns whether Integrations section is enabled in Settings runtime config. */
export function isIntegrationsEnabled(config = getRuntimeConfig()): boolean {
  return parseFlag(config.featureIntegrationsEnabled, false);
}

/** Returns whether Secrets management surface is enabled in the current runtime config. */
export function isSecretsEnabled(config = getRuntimeConfig()): boolean {
  return parseFlag(config.featureSecretsEnabled, false);
}

/** Returns whether a shell nav/runtime feature is visible in current config. */
export function isFeatureVisible(featureID: string, config = getRuntimeConfig()): boolean {
  if (featureID === 'tasks') {
    return isTasksEnabled(config);
  }
  if (featureID === 'feature-capability') {
    return isFeatureCapabilityEnabled(config);
  }
  if (featureID === 'chat') {
    return isChatEnabled(config);
  }
  if (featureID === 'integrations') {
    return isIntegrationsEnabled(config);
  }
  if (featureID === 'secrets') {
    return isSecretsEnabled(config);
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

/** Returns whether PRD diagnostic-level Resolve actions are enabled in Documents. */
export function isPRDDiagnosticResolveEnabled(config = getRuntimeConfig()): boolean {
  return parseFlag(config.featurePRDDiagnosticResolveEnabled, false);
}
