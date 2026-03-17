export const FEATURE_CAPABILITY_ACCESS_PERMISSION = 'feature_capability:access';

type RuntimeConfig = {
  featureCapabilityAccess?: boolean | string;
  operatorPermissions?: string[] | string;
  permissions?: string[] | string;
};

function parseBoolean(value: boolean | string): boolean | null {
  if (typeof value === 'boolean') {
    return value;
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

function getRuntimeConfig(): RuntimeConfig {
  if (typeof window === 'undefined') {
    return {};
  }
  return ((window as any).__SMITH_CONFIG__ || {}) as RuntimeConfig;
}

/** Returns whether the current operator can access the feature capability flow. */
export function hasFeatureCapabilityAccess(config = getRuntimeConfig()): boolean {
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
