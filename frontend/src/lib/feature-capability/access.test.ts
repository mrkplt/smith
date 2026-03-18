import { describe, expect, it } from 'vitest';

import { hasFeatureCapabilityAccess } from '$lib/feature-capability/access';

describe('feature capability access', () => {
  it('defaults to deny while feature flag is disabled', () => {
    expect(hasFeatureCapabilityAccess({})).toBe(false);
  });

  it('allows when feature flag is enabled and no explicit access signal exists', () => {
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true })).toBe(true);
  });

  it('supports explicit boolean override', () => {
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, featureCapabilityAccess: true })).toBe(true);
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, featureCapabilityAccess: false })).toBe(false);
  });

  it('supports explicit string override', () => {
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, featureCapabilityAccess: 'true' })).toBe(true);
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, featureCapabilityAccess: 'false' })).toBe(false);
  });

  it('checks permission arrays and csv strings when explicit override is not set', () => {
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, operatorPermissions: ['feature_capability:access'] })).toBe(true);
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, permissions: 'feature_capability:access,other:perm' })).toBe(true);
    expect(hasFeatureCapabilityAccess({ featureCapabilityEnabled: true, permissions: ['different:permission'] })).toBe(false);
  });

  it('prioritizes explicit override over permissions', () => {
    expect(hasFeatureCapabilityAccess({
      featureCapabilityEnabled: true,
      featureCapabilityAccess: false,
      permissions: ['feature_capability:access']
    })).toBe(false);
  });
});
