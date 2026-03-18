import { getRuntimeConfig, isFeatureCapabilityEnabled, parseBoolean } from '$lib/feature-flags';

export const FEATURE_CAPABILITY_ACCESS_PERMISSION = 'feature_capability:access';

type FeatureCapabilityRuntimeConfig = {
  featureCapabilityEnabled?: boolean | string;
  featureCapabilityAccess?: boolean | string;
  operatorPermissions?: string[] | string;
  permissions?: string[] | string;
};

function normalizePermissions(value: string[] | string | undefined): string[] {
  if (Array.isArray(value)) {
    return value.map((item) => item.trim()).filter((item) => item.length > 0);
  }
  if (typeof value === 'string') {
    return value
      .split(',')
      .map((item) => item.trim())
      .filter((item) => item.length > 0);
  }
  return [];
}

/** Returns whether the current operator can access the feature capability flow. */
export function hasFeatureCapabilityAccess(config: FeatureCapabilityRuntimeConfig = getRuntimeConfig() as FeatureCapabilityRuntimeConfig): boolean {
  if (!isFeatureCapabilityEnabled(config)) {
    return false;
  }

  if (config.featureCapabilityAccess !== undefined) {
    const parsed = parseBoolean(config.featureCapabilityAccess);
    if (parsed !== null) {
      return parsed;
    }
  }

  const permissions = [
    ...normalizePermissions(config.operatorPermissions),
    ...normalizePermissions(config.permissions)
  ];

  if (permissions.length === 0) {
    return true;
  }

  return permissions.includes(FEATURE_CAPABILITY_ACCESS_PERMISSION);
}
