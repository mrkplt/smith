import { describe, expect, it, vi } from 'vitest';

import {
  __resetFeatureFlagDiagnosticsForTests,
  isChatEnabled,
  isFeatureCapabilityEnabled,
  isFeatureVisible,
  isIntegrationsEnabled,
  isPRDDiagnosticResolveEnabled,
  isProviderTypeEnabled,
  isSecretsEnabled,
  isTasksKanbanEnabled,
  isTasksKanbanVisible,
  isTasksEnabled,
  parseBoolean
} from '$lib/feature-flags';

describe('feature flags', () => {
  it('emits a one-time non-sensitive diagnostic for tasks-kanban evaluation', () => {
    __resetFeatureFlagDiagnosticsForTests();
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});

    expect(isTasksKanbanEnabled({})).toBe(false);
    expect(isTasksKanbanEnabled({ featureTasksKanbanEnabled: true })).toBe(true);
    expect(infoSpy).toHaveBeenCalledTimes(1);
    expect(infoSpy.mock.calls[0][0]).toContain('feature=tasks-kanban enabled=false source=runtime-config');

    infoSpy.mockRestore();
  });

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
    expect(isTasksKanbanEnabled({})).toBe(false);
    expect(isFeatureCapabilityEnabled({})).toBe(false);
    expect(isChatEnabled({})).toBe(false);
    expect(isIntegrationsEnabled({})).toBe(false);
    expect(isSecretsEnabled({})).toBe(false);
  });

  it('supports boolean and string runtime overrides', () => {
    expect(isTasksEnabled({ featureTasksEnabled: true })).toBe(true);
    expect(isTasksEnabled({ featureTasksEnabled: 'true' })).toBe(true);
    expect(isTasksKanbanEnabled({ featureTasksKanbanEnabled: true })).toBe(true);
    expect(isTasksKanbanEnabled({ featureTasksKanbanEnabled: '1' })).toBe(true);
    expect(isFeatureCapabilityEnabled({ featureCapabilityEnabled: true })).toBe(true);
    expect(isFeatureCapabilityEnabled({ featureCapabilityEnabled: '1' })).toBe(true);
  });

  it('fails closed for invalid Tasks Kanban flag values', () => {
    expect(isTasksKanbanEnabled({ featureTasksKanbanEnabled: 'enabled' })).toBe(false);
    expect(isTasksKanbanEnabled({ featureTasksKanbanEnabled: '' })).toBe(false);
  });

  it('requires both Tasks and Tasks Kanban flags for Kanban visibility', () => {
    expect(isTasksKanbanVisible({})).toBe(false);
    expect(isTasksKanbanVisible({ featureTasksEnabled: true, featureTasksKanbanEnabled: false })).toBe(false);
    expect(isTasksKanbanVisible({ featureTasksEnabled: false, featureTasksKanbanEnabled: true })).toBe(false);
    expect(isTasksKanbanVisible({ featureTasksEnabled: true, featureTasksKanbanEnabled: true })).toBe(true);
  });

  it('maps feature ids to visibility defaults', () => {
    expect(isFeatureVisible('tasks', {})).toBe(false);
    expect(isFeatureVisible('feature-capability', {})).toBe(false);
    expect(isFeatureVisible('chat', {})).toBe(false);
    expect(isFeatureVisible('integrations', {})).toBe(false);
    expect(isFeatureVisible('secrets', {})).toBe(false);
    expect(isFeatureVisible('pods', {})).toBe(true);
  });

  it('supports runtime overrides for integrations feature gate', () => {
    expect(isIntegrationsEnabled({ featureIntegrationsEnabled: true })).toBe(true);
    expect(isIntegrationsEnabled({ featureIntegrationsEnabled: '1' })).toBe(true);
    expect(isIntegrationsEnabled({ featureIntegrationsEnabled: false })).toBe(false);
    expect(isIntegrationsEnabled({ featureIntegrationsEnabled: 'false' })).toBe(false);
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

  it('defaults codex/openai/claude provider types enabled', () => {
    expect(isProviderTypeEnabled('codex', {})).toBe(true);
    expect(isProviderTypeEnabled('openai', {})).toBe(true);
    expect(isProviderTypeEnabled('claude', {})).toBe(true);
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
