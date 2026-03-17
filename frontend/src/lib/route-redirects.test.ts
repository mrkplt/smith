import { describe, expect, it } from 'vitest';

import { load as rootLoad } from '../routes/+page';
import { load as projectsLoad } from '../routes/projects/+page';
import { load as providersLoad } from '../routes/providers/+page';

function buildRootLoadEvent(fetchImpl: any) {
  return {
    fetch: fetchImpl,
    data: {},
    params: {},
    route: { id: '/' },
    url: new URL('http://localhost/'),
    depends: () => {},
    parent: async () => ({}),
    untrack: (fn: any) => fn()
  } as any;
}

describe('legacy settings route redirects', () => {
  it('redirects /providers to Settings > Providers', () => {
    try {
      providersLoad();
      throw new Error('expected redirect to be thrown');
    } catch (err: any) {
      expect(err?.status).toBe(307);
      expect(err?.location).toBe('/settings?section=providers');
    }
  });

  it('redirects /projects to Settings > Projects', () => {
    try {
      projectsLoad();
      throw new Error('expected redirect to be thrown');
    } catch (err: any) {
      expect(err?.status).toBe(307);
      expect(err?.location).toBe('/settings?section=projects');
    }
  });
});

describe('root route onboarding redirect', () => {
  it('redirects to onboarding when readiness is incomplete', async () => {
    const event = buildRootLoadEvent(async () =>
      new Response(JSON.stringify({ ready: false }), {
        status: 200,
        headers: { 'content-type': 'application/json' }
      })
    );

    try {
      await rootLoad(event);
      throw new Error('expected redirect to be thrown');
    } catch (err: any) {
      expect(err?.status).toBe(307);
      expect(err?.location).toBe('/onboarding');
    }
  });

  it('redirects to pods when readiness is complete', async () => {
    const event = buildRootLoadEvent(async () =>
      new Response(JSON.stringify({ ready: true }), {
        status: 200,
        headers: { 'content-type': 'application/json' }
      })
    );

    try {
      await rootLoad(event);
      throw new Error('expected redirect to be thrown');
    } catch (err: any) {
      expect(err?.status).toBe(307);
      expect(err?.location).toBe('/pods');
    }
  });

  it('falls back to pods when readiness cannot be loaded', async () => {
    const event = buildRootLoadEvent(async () => {
      throw new Error('network down');
    });

    try {
      await rootLoad(event);
      throw new Error('expected redirect to be thrown');
    } catch (err: any) {
      expect(err?.status).toBe(307);
      expect(err?.location).toBe('/pods');
    }
  });
});
