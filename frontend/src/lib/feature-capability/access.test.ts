import { describe, expect, it } from 'vitest';

import { hasFeatureCapabilityAccess } from '$lib/feature-capability/access';

describe('feature capability access', () => {
  it('defaults to allow when no explicit access signal exists', () => {
    expect(hasFeatureCapabilityAccess({})).toBe(true);
  });

  it('supports explicit boolean override', () => {
    expect(hasFeatureCapabilityAccess({ featureCapabilityAccess: true })).toBe(true);
    expect(hasFeatureCapabilityAccess({ featureCapabilityAccess: false })).toBe(false);
  });

  it('supports explicit string override', () => {
    expect(hasFeatureCapabilityAccess({ featureCapabilityAccess: 'true' })).toBe(true);
    expect(hasFeatureCapabilityAccess({ featureCapabilityAccess: 'false' })).toBe(false);
  });

  it('checks permission arrays and csv strings when explicit override is not set', () => {
    expect(hasFeatureCapabilityAccess({ operatorPermissions: ['feature_capability:access'] })).toBe(true);
    expect(hasFeatureCapabilityAccess({ permissions: 'feature_capability:access,other:perm' })).toBe(true);
    expect(hasFeatureCapabilityAccess({ permissions: ['different:permission'] })).toBe(false);
  });

  it('prioritizes explicit override over permissions', () => {
    expect(hasFeatureCapabilityAccess({
      featureCapabilityAccess: false,
      permissions: ['feature_capability:access']
    })).toBe(false);
  });
});
