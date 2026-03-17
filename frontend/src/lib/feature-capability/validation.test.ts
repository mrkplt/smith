import { describe, expect, it } from 'vitest';

import { validateFeatureCapabilityInputs } from '$lib/feature-capability/validation';

describe('feature capability validation', () => {
  it('returns no errors for valid required inputs', () => {
    expect(validateFeatureCapabilityInputs({
      taskName: 'Run sync',
      targetEnvironment: 'staging'
    })).toEqual({});
  });

  it('returns field-level errors for missing required inputs', () => {
    expect(validateFeatureCapabilityInputs({
      taskName: ' ',
      targetEnvironment: ''
    })).toEqual({
      taskName: 'Task name is required.',
      targetEnvironment: 'Target environment is required.'
    });
  });
});
