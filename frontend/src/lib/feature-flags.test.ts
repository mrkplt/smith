import { describe, expect, it } from 'vitest';

import { isFeatureCapabilityEnabled, isFeatureVisible, isTasksEnabled, parseBoolean } from '$lib/feature-flags';

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
    expect(isFeatureVisible('pods', {})).toBe(true);
  });
});
