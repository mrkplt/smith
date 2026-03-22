import { describe, expect, it } from 'vitest';

import { shellPrimaryNav } from '$lib/navigation';

describe('shell navigation', () => {
  it('keeps runtime surfaces plus settings in primary nav', () => {
    expect(shellPrimaryNav.map((item) => item.id)).toEqual([
      'pods',
      'documents',
      'tasks-kanban',
      'feature-capability',
      'settings'
    ]);
    expect(shellPrimaryNav.map((item) => item.href)).toEqual([
      '/pods',
      '/documents',
      '/tasks/kanban',
      '/feature-capability',
      '/settings'
    ]);
  });
});
