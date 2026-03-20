import { describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  requestJSON: vi.fn()
}));

vi.mock('$lib/api', () => ({
  requestJSON: apiMocks.requestJSON
}));

import { inferPRDFormat, validatePRDContent } from '$lib/documents/prd-validation';

describe('PRD validation helpers', () => {
  it('infers json content from object-shaped text', () => {
    expect(inferPRDFormat('{"version":1}')).toBe('json');
    expect(inferPRDFormat(' # Markdown heading')).toBe('markdown');
  });

  it('posts markdown and json payloads to readiness endpoint', async () => {
    apiMocks.requestJSON.mockResolvedValue({ report: { valid: true, errors: [], warnings: [], readiness: 'pass' } });

    await validatePRDContent('# doc', 'markdown');
    await validatePRDContent('{"version":1}', 'json');

    expect(apiMocks.requestJSON).toHaveBeenNthCalledWith(1, '/v1/prd/validate', 'POST', {
      format: 'markdown',
      markdown: '# doc'
    });
    expect(apiMocks.requestJSON).toHaveBeenNthCalledWith(2, '/v1/prd/validate', 'POST', {
      format: 'json',
      json: '{"version":1}'
    });
  });
});
