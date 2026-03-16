import { describe, expect, it } from 'vitest';

import {
  buildChatPageURL,
  buildGlobalChatContext,
  contextSignature,
  extractCarriedChatContext,
  sanitizeReturnToPath
} from '$lib/chat/context';

const baseState = {
  activeProvider: '',
  documents: [],
  loops: [],
  projects: [],
  selectedLoop: ''
};

describe('chat context helpers', () => {
  it('builds route-aware context for pod view pages', () => {
    const context = buildGlobalChatContext('/pod-view/loop-123', 'prd-refinement', {
      ...baseState,
      loops: [{ loopID: 'loop-123', project: 'proj-9', status: 'running' }],
      projects: [{ id: 'proj-9' }]
    } as any);

    expect(context.surface).toBe('pod-view');
    expect(context.loopId).toBe('loop-123');
    expect(context.projectId).toBe('proj-9');
    expect(context.sessionType).toBe('prd-refinement');
  });

  it('falls back to selected loop on pods page', () => {
    const context = buildGlobalChatContext('/pods', 'prd-refinement', {
      ...baseState,
      selectedLoop: 'loop-777'
    } as any);

    expect(context.surface).toBe('pods');
    expect(context.loopId).toBe('loop-777');
  });

  it('marks settings route surface correctly', () => {
    const context = buildGlobalChatContext('/settings', 'prd-refinement', baseState as any);
    expect(context.surface).toBe('settings');
  });

  it('round-trips carried context through chat URL', () => {
    const input = {
      app: 'smith-console',
      route: '/projects',
      surface: 'projects',
      provider: 'openai',
      model: 'gpt-5-mini',
      providerApiKey: 'sk-secret'
    };

    const url = buildChatPageURL(input, '/projects');
    const query = new URL(url, 'http://localhost').searchParams;
    const output = extractCarriedChatContext(query);

    expect(output).toEqual({
      app: 'smith-console',
      route: '/projects',
      surface: 'projects',
      provider: 'openai',
      model: 'gpt-5-mini'
    });
    expect(output.providerApiKey).toBeUndefined();
  });

  it('sanitizes invalid return paths', () => {
    expect(sanitizeReturnToPath('')).toBe('/projects');
    expect(sanitizeReturnToPath('projects')).toBe('/projects');
    expect(sanitizeReturnToPath('/assistant')).toBe('/projects');
    expect(sanitizeReturnToPath('/documents')).toBe('/documents');
  });
});
