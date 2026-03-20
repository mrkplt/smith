<script lang="ts">
  import { Badge, Button } from 'flowbite-svelte';
  import type { PRDValidationDiagnostic, PRDValidationReport } from '$lib/documents/prd-validation';

  interface Props {
    report: PRDValidationReport | null;
    busy: boolean;
    errorMessage: string;
    format: 'markdown' | 'json';
    chatEnabled: boolean;
    resolveDiagnosticEnabled: boolean;
    onRecheck: () => void;
    onRefineWithAI: () => void;
    onResolveDiagnostic: (diagnostic: PRDValidationDiagnostic) => void;
  }

  let {
    report,
    busy,
    errorMessage,
    format,
    chatEnabled,
    resolveDiagnosticEnabled,
    onRecheck,
    onRefineWithAI,
    onResolveDiagnostic
  }: Props = $props();
  let showNotifications = $state(false);

  type NotificationSeverity = 'error' | 'warning';

  interface NotificationItem extends PRDValidationDiagnostic {
    severity: NotificationSeverity;
  }

  function badgeColor(readiness: string): 'green' | 'yellow' | 'red' {
    if (readiness === 'pass') {
      return 'green';
    }
    if (readiness === 'warn') {
      return 'yellow';
    }
    return 'red';
  }

  const notificationItems = $derived.by((): NotificationItem[] => {
    if (!report) {
      return [];
    }
    const errors = (report.errors || []).map((item) => ({ ...item, severity: 'error' as const }));
    const warnings = (report.warnings || []).map((item) => ({ ...item, severity: 'warning' as const }));
    return [...errors, ...warnings];
  });

  const notificationCount = $derived(notificationItems.length);

  const readinessLabel = $derived.by(() => {
    if (!report) {
      return '';
    }
    if (report.readiness !== 'pass' && notificationCount > 0) {
      return `${report.readiness} (${notificationCount})`;
    }
    return report.readiness;
  });

  function toggleNotifications() {
    if (!report || notificationCount === 0) {
      showNotifications = false;
      return;
    }
    showNotifications = !showNotifications;
  }

  function closeNotifications() {
    showNotifications = false;
  }

  function runResolveDiagnostic(diagnostic: PRDValidationDiagnostic) {
    closeNotifications();
    onResolveDiagnostic(diagnostic);
  }

  $effect(() => {
    if (!report || busy) {
      showNotifications = false;
    }
  });
</script>

<section class="prd-validation border-b border-gray-900 px-8 py-4 relative">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="flex items-center gap-2">
      <h2 class="text-[11px] uppercase font-bold tracking-[0.16em] text-gray-400">PRD Readiness</h2>
      <span class="text-[10px] uppercase tracking-[0.14em] text-gray-600">{format}</span>
      {#if busy}
        <span class="text-[10px] uppercase tracking-[0.14em] text-gray-500">Checking...</span>
      {/if}
    </div>
    <div class="actions-wrap flex items-center gap-2">
      {#if report}
        <button class="readiness-indicator" data-readiness={report.readiness} onclick={toggleNotifications}>
          <Badge color={badgeColor(report.readiness)} class="rounded-none uppercase text-[9px] tracking-[0.12em]">{readinessLabel}</Badge>
        </button>
      {/if}
      <Button color="alternative" class="rounded-none border-gray-700 text-[9px] uppercase tracking-[0.14em] px-3 h-7" onclick={() => onRecheck()}>Recheck</Button>
      {#if chatEnabled}
        <Button color="alternative" class="rounded-none bg-[#86BC25] text-black text-[9px] uppercase tracking-[0.14em] px-3 h-7" onclick={() => onRefineWithAI()}>Refine with AI</Button>
      {/if}

      {#if showNotifications && report}
        <button type="button" class="overlay-backdrop" aria-label="Close notifications" onclick={closeNotifications}></button>
        <div class="notification-overlay">
          <div class="notification-overlay-title">Notifications ({notificationCount})</div>
          {#each notificationItems as diagnostic}
            <div class="notification-row">
              <span class="notification-severity" data-severity={diagnostic.severity}>
                {diagnostic.severity}
              </span>
              <div class="notification-content">
                <div class="diagnostic-code">{diagnostic.code}</div>
                <div class="diagnostic-message">{diagnostic.message}</div>
                {#if diagnostic.suggestion}
                  <div class="diagnostic-suggestion">Tip: {diagnostic.suggestion}</div>
                {/if}
              </div>
              {#if chatEnabled && resolveDiagnosticEnabled}
                <div class="notification-actions">
                  <button type="button" class="notification-action" onclick={() => runResolveDiagnostic(diagnostic)}>Resolve</button>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  {#if errorMessage}
    <p class="mt-3 text-xs text-red-300">{errorMessage}</p>
  {:else if report}
    <p class="mt-3 text-xs text-gray-500">
      {#if notificationCount > 0}
        {notificationCount} readiness notification{notificationCount === 1 ? '' : 's'} hidden. Click the readiness badge to open.
      {:else}
        No readiness notifications.
      {/if}
    </p>
  {:else}
    <p class="mt-3 text-xs text-gray-500">Start editing to see readiness diagnostics and concrete suggestions.</p>
  {/if}
</section>

<style>
  .prd-validation {
    background: #040404;
  }

  .actions-wrap {
    position: relative;
  }

  .readiness-indicator {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    border: 1px solid rgba(255, 255, 255, 0.12);
    padding: 3px 4px;
    background: rgba(255, 255, 255, 0.03);
    cursor: pointer;
  }

  .readiness-indicator:hover {
    border-color: rgba(255, 255, 255, 0.2);
    background: rgba(255, 255, 255, 0.08);
  }

  .overlay-backdrop {
    position: fixed;
    inset: 0;
    z-index: 20;
  }

  .notification-overlay {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    width: min(620px, calc(100vw - 80px));
    max-height: 48vh;
    overflow: auto;
    border: 1px solid rgba(255, 255, 255, 0.14);
    background: rgba(6, 8, 12, 0.98);
    padding: 8px 10px 10px;
    z-index: 30;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.45);
  }

  .notification-overlay-title {
    font-size: 0.62rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0.14em;
    color: #93a1b5;
    margin-bottom: 4px;
  }

  .notification-row {
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    padding-top: 7px;
    margin-top: 7px;
    display: flex;
    gap: 10px;
    align-items: flex-start;
  }

  .notification-row:hover .notification-actions {
    opacity: 1;
  }

  .notification-severity {
    margin-top: 2px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    color: #9ba7b8;
    font-size: 0.55rem;
    line-height: 1;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    padding: 3px 5px;
    height: fit-content;
  }

  .notification-severity[data-severity='error'] {
    border-color: rgba(239, 68, 68, 0.45);
    color: #fca5a5;
    background: rgba(127, 29, 29, 0.35);
  }

  .notification-severity[data-severity='warning'] {
    border-color: rgba(245, 158, 11, 0.45);
    color: #fcd34d;
    background: rgba(120, 53, 15, 0.35);
  }

  .notification-content {
    min-width: 0;
    flex: 1;
  }

  .notification-actions {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    opacity: 0;
    transition: opacity 120ms ease;
    padding-top: 2px;
  }

  .notification-action {
    border: 1px solid rgba(255, 255, 255, 0.2);
    background: rgba(255, 255, 255, 0.02);
    color: #d5deea;
    font-size: 0.58rem;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    padding: 4px 6px;
    line-height: 1;
  }

  .notification-action:hover {
    border-color: rgba(134, 188, 37, 0.6);
    color: #a8d756;
  }

  .diagnostic-code {
    color: #86bc25;
    font-size: 0.6rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    font-family: var(--mono);
  }

  .diagnostic-message {
    color: #d6dce6;
    font-size: 0.78rem;
    margin-top: 4px;
  }

  .diagnostic-suggestion {
    color: #9ba7b8;
    font-size: 0.72rem;
    margin-top: 4px;
    line-height: 1.4;
  }

</style>
