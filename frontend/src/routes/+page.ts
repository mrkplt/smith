import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
  let destination = '/pods';
  try {
    const response = await fetch('/api/v1/onboarding/readiness', {
      headers: { Accept: 'application/json' }
    });
    if (response.ok) {
      const readiness = await response.json();
      if (readiness && readiness.ready === false) {
        destination = '/onboarding';
      }
    }
  } catch {
    destination = '/pods';
  }

  throw redirect(307, destination);
};
