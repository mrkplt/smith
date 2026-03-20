<script lang="ts">
  import { Badge, Button } from 'flowbite-svelte';
  import type { PRDValidationDiagnostic, PRDValidationReport } from '$lib/documents/prd-validation';

  interface Props {
    report: PRDValidationReport | null;
    busy: boolean;
    errorMessage: string;
    format: 'markdown' | 'json';
    onRecheck: () => void;
    onRefineWithAI: () => void;
  }

  let { report, busy, errorMessage, format, onRecheck, onRefineWithAI }: Props = $props();

  function badgeColor(readiness: string): 'green' | 'yellow' | 'red' {
    if (readiness === 'pass') {
      return 'green';
    }
    if (readiness === 'warn') {
      return 'yellow';
    }
    return 'red';
  }

  function visibleDiagnostics(items: PRDValidationDiagnostic[]): PRDValidationDiagnostic[] {
    return items.slice(0, 6);
  }
</script>

<section class="prd-validation border-b border-gray-900 px-8 py-4">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <div class="flex items-center gap-2">
      <h2 class="text-[11px] uppercase font-bold tracking-[0.16em] text-gray-400">PRD Readiness</h2>
      <span class="text-[10px] uppercase tracking-[0.14em] text-gray-600">{format}</span>
      {#if busy}
        <span class="text-[10px] uppercase tracking-[0.14em] text-gray-500">Checking...</span>
      {:else if report}
        <Badge color={badgeColor(report.readiness)} class="rounded-none uppercase text-[9px] tracking-[0.12em]">{report.readiness}</Badge>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      <Button color="alternative" class="rounded-none border-gray-700 text-[9px] uppercase tracking-[0.14em] px-3 h-7" onclick={onRecheck}>Recheck</Button>
      <Button color="alternative" class="rounded-none bg-[#86BC25] text-black text-[9px] uppercase tracking-[0.14em] px-3 h-7" onclick={onRefineWithAI}>Refine with AI</Button>
    </div>
  </div>

  {#if errorMessage}
    <p class="mt-3 text-xs text-red-300">{errorMessage}</p>
  {:else if report}
    <div class="mt-3 grid gap-3 md:grid-cols-2">
      <div class="diagnostic-card">
        <div class="diagnostic-title">Errors ({report.errors.length})</div>
        {#if report.errors.length === 0}
          <p class="diagnostic-empty">No blocking errors.</p>
        {:else}
          {#each visibleDiagnostics(report.errors) as diagnostic}
            <div class="diagnostic-row">
              <div class="diagnostic-code">{diagnostic.code}</div>
              <div class="diagnostic-message">{diagnostic.message}</div>
              {#if diagnostic.suggestion}
                <div class="diagnostic-suggestion">Tip: {diagnostic.suggestion}</div>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
      <div class="diagnostic-card">
        <div class="diagnostic-title">Warnings ({report.warnings.length})</div>
        {#if report.warnings.length === 0}
          <p class="diagnostic-empty">No warnings.</p>
        {:else}
          {#each visibleDiagnostics(report.warnings) as diagnostic}
            <div class="diagnostic-row">
              <div class="diagnostic-code">{diagnostic.code}</div>
              <div class="diagnostic-message">{diagnostic.message}</div>
              {#if diagnostic.suggestion}
                <div class="diagnostic-suggestion">Tip: {diagnostic.suggestion}</div>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
    </div>
  {:else}
    <p class="mt-3 text-xs text-gray-500">Start editing to see readiness diagnostics and concrete suggestions.</p>
  {/if}
</section>

<style>
  .prd-validation {
    background: #040404;
  }

  .diagnostic-card {
    border: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(8, 8, 8, 0.8);
    padding: 10px;
  }

  .diagnostic-title {
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.18em;
    color: #8b98ab;
    margin-bottom: 8px;
    font-weight: 800;
  }

  .diagnostic-empty {
    margin: 0;
    font-size: 0.75rem;
    color: #7d8898;
  }

  .diagnostic-row {
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    padding-top: 8px;
    margin-top: 8px;
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
