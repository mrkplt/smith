import { describe, expect, it } from 'vitest';

import {
  isChatEnabled,
  isFeatureCapabilityEnabled,
  isFeatureVisible,
  isPRDDiagnosticResolveEnabled,
  isProviderTypeEnabled,
  isSecretsEnabled,
  isTasksEnabled,
  parseBoolean
} from '$lib/feature-flags';

describe('feature flags', () => {
  it('parses booleans consistently', () => {
    expect(parseBoolean(true)).toBe(true);
    expect(parseBoolean(false)).toBe(false);
    expect(parseBoolean('true')).toBe(true);
    expect(parseBoolean('1')).toBe(true);
    expect(parseBoolean('yes')).toBe(true);
    expect(parseBoolean('false')).toBe(false);
    expect(parseBoolean('0')).toBe(false);
    expect(parseBoolean('no')).toBe(false);
    expect(parseBoolean('')).toBe(null);
    expect(parseBoolean(undefined)).toBe(null);
  });

  it('defaults unfinished features off', () => {
    expect(isTasksEnabled({})).toBe(false);
    expect(isFeatureCapabilityEnabled({})).toBe(false);
    expect(isChatEnabled({})).toBe(false);
    expect(isSecretsEnabled({})).toBe(false);
  });

  it('supports boolean and string runtime overrides', () => {
    expect(isTasksEnabled({ featureTasksEnabled: true })).toBe(true);
    expect(isTasksEnabled({ featureTasksEnabled: 'true' })).toBe(true);
    expect(isFeatureCapabilityEnabled({ featureCapabilityEnabled: true })).toBe(true);
    expect(isFeatureCapabilityEnabled({ featureCapabilityEnabled: '1' })).toBe(true);
  });

  it('maps feature ids to visibility defaults', () => {
    expect(isFeatureVisible('tasks', {})).toBe(false);
    expect(isFeatureVisible('feature-capability', {})).toBe(false);
    expect(isFeatureVisible('chat', {})).toBe(false);
    expect(isFeatureVisible('secrets', {})).toBe(false);
    expect(isFeatureVisible('pods', {})).toBe(true);
  });

  it('supports runtime overrides for chat feature gate', () => {
    expect(isChatEnabled({ featureChatEnabled: true })).toBe(true);
    expect(isChatEnabled({ featureChatEnabled: '1' })).toBe(true);
    expect(isChatEnabled({ featureChatEnabled: false })).toBe(false);
    expect(isChatEnabled({ featureChatEnabled: 'false' })).toBe(false);
  });

  it('supports runtime overrides for secrets feature gate', () => {
    expect(isSecretsEnabled({ featureSecretsEnabled: true })).toBe(true);
    expect(isSecretsEnabled({ featureSecretsEnabled: '1' })).toBe(true);
    expect(isSecretsEnabled({ featureSecretsEnabled: false })).toBe(false);
    expect(isSecretsEnabled({ featureSecretsEnabled: 'false' })).toBe(false);
  });

  it('defaults only codex/openai provider enabled', () => {
    expect(isProviderTypeEnabled('codex', {})).toBe(true);
    expect(isProviderTypeEnabled('openai', {})).toBe(true);
    expect(isProviderTypeEnabled('claude', {})).toBe(false);
    expect(isProviderTypeEnabled('gemini', {})).toBe(false);
  });

  it('supports runtime overrides for provider feature gates', () => {
    const config = {
      featureProviderClaudeEnabled: true,
      featureProviderGeminiEnabled: '1'
    };
    expect(isProviderTypeEnabled('claude', config)).toBe(true);
    expect(isProviderTypeEnabled('anthropic', config)).toBe(true);
    expect(isProviderTypeEnabled('gemini', config)).toBe(true);
    expect(isProviderTypeEnabled('google', config)).toBe(true);
  });

  it('defaults PRD diagnostic resolve feature off', () => {
    expect(isPRDDiagnosticResolveEnabled({})).toBe(false);
  });

  it('supports PRD diagnostic resolve runtime override', () => {
    expect(isPRDDiagnosticResolveEnabled({ featurePRDDiagnosticResolveEnabled: true })).toBe(true);
    expect(isPRDDiagnosticResolveEnabled({ featurePRDDiagnosticResolveEnabled: '1' })).toBe(true);
    expect(isPRDDiagnosticResolveEnabled({ featurePRDDiagnosticResolveEnabled: 'false' })).toBe(false);
  });
});
