import { beforeEach, describe, expect, it, vi } from 'vitest';

import * as api from '$lib/api';
import { clearProviderModelsCache, loadProviderModels, staticModelsForProviderType } from '$lib/providers/models';

vi.mock('$lib/api', () => ({
  fetchJSON: vi.fn()
}));

describe('provider model loading', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearProviderModelsCache();
  });

  it('skips dynamic model lookup when provider credentials are missing', async () => {
    const models = await loadProviderModels('claude-default', 'claude', false);

    expect(models).toEqual(staticModelsForProviderType('claude'));
    expect(api.fetchJSON).not.toHaveBeenCalled();
  });

  it('does not cache static fallback from missing credentials', async () => {
    await loadProviderModels('claude-default', 'claude', false);
    vi.mocked(api.fetchJSON).mockResolvedValue({
      provider_type: 'claude',
      default_model: 'claude-sonnet-4-5',
      models: [{ id: 'claude-opus-4-1' }]
    });

    const models = await loadProviderModels('claude-default', 'claude', true);

    expect(api.fetchJSON).toHaveBeenCalledWith('/v1/providers/claude-default/models');
    expect(models).toContain('claude-opus-4-1');
  });
});
