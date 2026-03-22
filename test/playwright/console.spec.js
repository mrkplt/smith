import { expect, test } from '@playwright/test';
import {
  mockEventSource,
  mockApiRoutes,
  emitLoopUpdates,
  emitDocumentUpdates,
  emitChatEvent,
  loopsFixture,
  documentsFixture,
  defaultProvidersFixture,
} from './helpers.js';

// ── helpers ────────────────────────────────────────────────────────

async function setupPods(page) {
  await mockEventSource(page);
  const api = await mockApiRoutes(page);
  await page.goto('/pods');
  await emitLoopUpdates(page, loopsFixture);
  // Wait for at least one pod card to render
  await expect(page.locator('.pod-card-container').first()).toBeVisible();
  return api;
}

// ── tests ──────────────────────────────────────────────────────────

test('renders loop tiles and summary stats', async ({ page }) => {
  await setupPods(page);

  await expect(page.locator('.grid > div').filter({ hasText: 'Total Pods' }).locator('span.text-3xl')).toHaveText('3');
  await expect(page.locator('.pod-card-container')).toHaveCount(2);
});

test('pod detail and command execution', async ({ page }) => {
  const api = await setupPods(page);

  // Click first tile (loop-alpha)
  await page.locator('.pod-card-container', { hasText: 'loop-alpha' }).click();
  await expect(page).toHaveURL(/\/pod-view\/loop-alpha/);

  // Verify title
  await expect(page.locator('h1')).toContainText('Pod: loop-alpha');

  // Fill command input and submit
  await page.getByPlaceholder('Run command').fill('echo ok');
  await page.keyboard.press('Enter');

  // Verify command was sent
  await expect.poll(() => api.commandPayloads.length).toBe(1);
  expect(api.commandPayloads[0].command).toBe('echo ok');
});

test('cancel and terminate from pod detail', async ({ page }) => {
  const api = await setupPods(page);

  // Navigate to pod-view for loop-beta
  await page.locator('.pod-card-container', { hasText: 'loop-beta' }).click();
  await expect(page).toHaveURL(/\/pod-view\/loop-beta/);

  // Click Cancel button
  await page.getByRole('button', { name: 'Cancel' }).click();

  await expect.poll(() => api.cancelPayloads.length).toBe(1);
  expect(api.cancelPayloads[0].reason).toBe('cancelled via console');

  // Click Terminate button
  await page.getByRole('button', { name: 'Terminate' }).click();

  await expect.poll(() => api.overridePayloads.length).toBe(1);
  expect(api.overridePayloads[0].target_state).toBe('flatline');
});

test('filters by state and search', async ({ page }) => {
  await setupPods(page);

  // Filter by search
  await page.getByPlaceholder('Filter ID...').fill('alpha');
  await expect(page.locator('.pod-card-container')).toHaveCount(1);

  // Clear search
  await page.getByPlaceholder('Filter ID...').fill('');
  await expect(page.locator('.pod-card-container')).toHaveCount(2);

  // Filter by state
  await page.locator('select').selectOption('flatline');
  await expect(page.locator('.pod-card-container')).toHaveCount(1);
  await expect(page.locator('.pod-card-container')).toContainText('loop-gamma');
});

test('provider API key config', async ({ page }) => {
  await mockEventSource(page);
  const api = await mockApiRoutes(page, { providers: defaultProvidersFixture });

  await page.goto('/providers');

  // Unconfigured defaults are hidden until they have a credential label.
  await expect(page.getByText('No Provider Profiles')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Configure' })).toHaveCount(0);

  // Create a reusable profile + credential from the providers section.
  await page.getByRole('button', { name: 'New Profile' }).click();
  await page.getByTestId('provider-profile-id').fill('codex-work');
  await page.getByTestId('provider-display-name').fill('Codex Work');
  await page.getByTestId('provider-credential-id').fill('codex-work-key');
  await page.getByTestId('provider-api-key').fill('sk-test-key');
  await page.getByTestId('provider-save').click();

  // Verify provider and secret state were updated and now visible in UI.
  await expect.poll(() => api.providersState.find((provider) => provider.id === 'codex-work')?.secret_ref).toBe('codex-work-key');
  await expect.poll(() => api.secretsState.some((secret) => secret.id === 'codex-work-key')).toBe(true);
  await expect(page.getByText('Codex Work')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Configure' }).first()).toBeVisible();
});

test('project management', async ({ page }) => {
  await mockEventSource(page);
  const api = await mockApiRoutes(page);

  await page.goto('/projects');

  // Verify existing project is shown
  await expect(page.locator('body')).toContainText('alpha');

  // Click New Project button
  await page.getByRole('button', { name: 'New Project' }).click();

  // Fill project form
  await page.locator('#name').fill('new-project');
  await page.locator('#repo').fill('https://github.com/org/repo.git');

  // Submit
  await page.getByRole('button', { name: 'Create Project' }).click();

  // Verify project was created via API
  await expect.poll(() => api.projectsState.find(p => p.name === 'new-project')).toBeDefined();
});

test('document refinement chat shows context and patch workflow', async ({ page }) => {
  await mockEventSource(page);
  const api = await mockApiRoutes(page, { documents: documentsFixture });

  await page.goto('/documents');
  await emitDocumentUpdates(page, documentsFixture);

  await page.locator('.doc-item', { hasText: 'Checkout PRD' }).click();
  await expect(page.getByRole('heading', { name: 'Checkout PRD' })).toBeVisible();

  await page.getByRole('button', { name: 'EDIT' }).click();

  await page.getByRole('button', { name: 'Refine with AI' }).click();

  await expect(page.locator('#documents-prd-chat-drawer')).toBeVisible();
  await expect.poll(() => api.getLastChatSessionID()).not.toBe('');

  const sessionID = api.getLastChatSessionID();
  await emitChatEvent(page, sessionID, 'context.loaded', {
    sessionIntent: 'document_refinement',
    documentTitle: 'Checkout PRD',
    documentVersion: '2026-03-18T12:00:00Z',
  });
  await emitChatEvent(page, sessionID, 'readiness.updated', { status: 'warn' });
  await emitChatEvent(page, sessionID, 'document.patch.proposed', {
    type: 'document_patch_proposal',
    operations: [
      {
        op: 'replace_document',
        content: '# Revised heading\n\n## Acceptance Criteria\n- retries are bounded\n',
      },
    ],
  });
  await emitChatEvent(page, sessionID, 'message.delta', { delta: 'Patch proposed for review.' });
  await emitChatEvent(page, sessionID, 'message.completed', {});

  await page.locator('.line-canvas .rendered-line-row', { hasText: 'Acceptance Criteria' }).click();

  await expect.poll(() => api.contextUpdatePayloads.length).toBeGreaterThan(0);
  await expect.poll(() => api.contextUpdatePayloads.some((payload) => payload?.focusContext?.sectionId === 'acceptance_criteria')).toBe(true);

  await expect(page.locator('#documents-prd-chat-drawer')).toContainText('Readiness: warn');
  await expect(page.locator('#documents-prd-chat-drawer')).toContainText('Patch Proposed');

  await page.getByRole('button', { name: 'Accept Patch' }).click();
  await expect(page.locator('#documents-prd-chat-drawer')).toBeHidden();
  await expect(page.locator('.line-canvas')).toContainText('Revised heading');
});
