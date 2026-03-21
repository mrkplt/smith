import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

import { appState } from '$lib/stores';
import * as api from '$lib/api';
import SettingsPage from './+page.svelte';

vi.mock('$app/state', () => ({
  page: {
    url: new URL('http://localhost/settings?section=providers')
  }
}));

vi.mock('$app/navigation', () => ({
  goto: vi.fn()
}));

vi.mock('$lib/api', () => ({
  apiBaseUrl: 'http://localhost:8080',
  chatBaseUrl: 'http://localhost:8080',
  fetchJSON: vi.fn(),
  postJSON: vi.fn(),
  requestJSON: vi.fn(),
  deleteJSON: vi.fn()
}));

describe('Settings providers next step CTA', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as any).__SMITH_CONFIG__ = {};
    window.localStorage.clear();
    appState.update((state) => ({
      ...state,
      projects: [],
      onboardingReady: false,
      onboardingChecked: false,
      onboardingState: null
    }));
    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/v1/providers') {
        return [
          {
            id: 'codex-default',
            name: 'Codex Default',
            provider_type: 'codex',
            default_model: 'gpt-5-codex',
            secret_ref: 'openai-key'
          }
        ];
      }
      if (path === '/v1/onboarding/readiness') {
        return { ready: true };
      }
      return [];
    });
  });

  it('keeps Add Project guidance when no projects are configured', async () => {
    render(SettingsPage);

    await waitFor(() => {
      expect(
        screen.getByText('Provider setup is complete. Continue to project setup to unlock loop execution.')
      ).toBeTruthy();
    });
    expect(screen.getByRole('button', { name: 'Add Project' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Open Pods' })).toBeNull();
  });

  it('hides next step guidance when at least one project is configured', async () => {
    appState.update((state) => ({
      ...state,
      projects: [
        {
          id: 'project-a',
          name: 'Project A',
          repo_url: 'https://github.com/acme/project-a'
        }
      ]
    }));

    render(SettingsPage);

    await waitFor(() => {
      expect(screen.getByText('Codex Default')).toBeTruthy();
    });
    expect(screen.queryByText('Next Step')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Add Project' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Open Pods' })).toBeNull();
  });

  it('shows only added provider profiles with credentials', async () => {
    vi.mocked(api.fetchJSON).mockImplementation(async (path: string) => {
      if (path === '/v1/providers') {
        return [
          {
            id: 'codex-default',
            name: 'Codex Default',
            provider_type: 'codex',
            default_model: 'gpt-5-codex',
            secret_ref: ''
          }
        ];
      }
      if (path === '/v1/onboarding/readiness') {
        return { ready: true };
      }
      return [];
    });

    render(SettingsPage);

    await waitFor(() => {
      expect(screen.getByText('No Provider Profiles')).toBeTruthy();
    });
    expect(screen.queryByText('Codex Default')).toBeNull();
  });
});
