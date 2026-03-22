<script lang="ts">
  import { Input } from 'flowbite-svelte';
  import { ChevronRightOutline } from 'flowbite-svelte-icons';

  interface Props {
    selectedStates: string[];
    searchQuery: string;
    onSelectedStatesChange: (value: string[]) => void;
    onSearchQueryChange: (value: string) => void;
  }

  let { selectedStates, searchQuery, onSelectedStatesChange, onSearchQueryChange }: Props = $props();
  let stateMenuOpen = $state(false);
  let stateMenuWrapEl: HTMLDivElement | null = null;

  const STATE_FILTER_OPTIONS = [
    { value: 'unresolved', label: 'Unresolved' },
    { value: 'running', label: 'Running' },
    { value: 'synced', label: 'Synced' },
    { value: 'flatline', label: 'Flatline' },
    { value: 'cancelled', label: 'Cancelled' }
  ];

  const normalizedSelectedStates = $derived(selectedStates.map((value) => value.toLowerCase()));

  const selectedStateLabel = $derived.by(() => {
    const healthyOnly = normalizedSelectedStates.length === 2 && normalizedSelectedStates.includes('running') && normalizedSelectedStates.includes('synced');
    if (healthyOnly) {
      return 'Healthy (Run + Done)';
    }
    if (normalizedSelectedStates.length === 0) {
      return 'All States';
    }
    if (normalizedSelectedStates.length === 1) {
      return STATE_FILTER_OPTIONS.find((option) => option.value === normalizedSelectedStates[0])?.label || '1 State';
    }
    return `${normalizedSelectedStates.length} States`;
  });

  function toggleState(value: string) {
    const normalized = value.toLowerCase();
    const next = normalizedSelectedStates.includes(normalized)
      ? normalizedSelectedStates.filter((item) => item !== normalized)
      : [...normalizedSelectedStates, normalized];
    onSelectedStatesChange(next);
  }

  function clearStates() {
    onSelectedStatesChange([]);
  }

  function handleWindowPointerDown(event: MouseEvent) {
    if (!stateMenuOpen || !stateMenuWrapEl) {
      return;
    }
    const target = event.target;
    if (!(target instanceof Node)) {
      return;
    }
    if (!stateMenuWrapEl.contains(target)) {
      stateMenuOpen = false;
    }
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      stateMenuOpen = false;
    }
  }
</script>

<svelte:window onmousedown={handleWindowPointerDown} onkeydown={handleWindowKeydown} />

<div class="pods-filter-bar">
  <div class="pods-filter-controls">
    <div class="pods-select-wrap" bind:this={stateMenuWrapEl}>
      <button
        type="button"
        class="smith-filter-control pods-filter-trigger"
        aria-haspopup="menu"
        aria-expanded={stateMenuOpen}
        aria-label="Filter pods by state"
        data-testid="pods-state-filter-trigger"
        onclick={() => (stateMenuOpen = !stateMenuOpen)}
      >
        <span class="pods-filter-label">{selectedStateLabel}</span>
        <ChevronRightOutline class={`pods-filter-chevron ${stateMenuOpen ? 'open' : ''}`} size="xs" />
      </button>

      {#if stateMenuOpen}
        <button type="button" class="overlay-backdrop" aria-label="Close pod state filters" onclick={() => (stateMenuOpen = false)}></button>
        <div class="pods-options-menu" role="menu" aria-label="Pod state filters" data-testid="pods-state-filter-menu">
          <div class="pods-options-head">
            <span class="pods-options-title">Filter States</span>
            <button type="button" class="pods-options-clear" onclick={clearStates}>Clear</button>
          </div>

          {#each STATE_FILTER_OPTIONS as option}
            <button
              type="button"
              class="pods-option"
              role="menuitemcheckbox"
              aria-checked={normalizedSelectedStates.includes(option.value)}
              onclick={() => toggleState(option.value)}
            >
              <span class="pods-option-checkbox" class:selected={normalizedSelectedStates.includes(option.value)}>
                {#if normalizedSelectedStates.includes(option.value)}
                  <span class="pods-option-checkmark">✓</span>
                {/if}
              </span>
              <span class="pods-option-label">{option.label}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <div class="pods-input-wrap">
      <Input
        class="smith-filter-control pods-input"
        type="search"
        placeholder="Filter pods..."
        value={searchQuery}
        oninput={(event) => onSearchQueryChange((event.currentTarget as HTMLInputElement).value)}
        size="sm"
      />
    </div>
  </div>
</div>

<style>
  .pods-filter-bar {
    display: flex;
    justify-content: flex-end;
    position: relative;
    z-index: 20;
  }

  .pods-filter-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .pods-select-wrap {
    width: 14rem;
  }

  .pods-input-wrap {
    width: 12rem;
  }

  .pods-filter-trigger {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.55rem;
    cursor: pointer;
    color: #334155;
    padding: 0 0.55rem;
  }

  .pods-filter-label {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  :global(.pods-filter-chevron) {
    color: #64748b;
    transition: transform 120ms ease;
    transform: rotate(90deg);
  }

  :global(.pods-filter-chevron.open) {
    transform: rotate(0deg);
  }

  .overlay-backdrop {
    position: fixed;
    inset: 0;
    z-index: 40;
    background: transparent;
    border: 0;
    padding: 0;
  }

  .pods-options-menu {
    position: absolute;
    top: calc(100% + 0.3rem);
    right: 0;
    width: min(20rem, calc(100vw - 3rem));
    border: 1px solid rgba(148, 163, 184, 0.4);
    background: rgba(255, 255, 255, 0.98);
    z-index: 50;
    padding: 0.5rem;
    display: grid;
    gap: 0.2rem;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.45);
  }

  .pods-options-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    padding-bottom: 0.35rem;
    margin-bottom: 0.15rem;
  }

  .pods-options-title {
    font-size: 0.6rem;
    text-transform: uppercase;
    letter-spacing: 0.14em;
    font-weight: 800;
    color: #64748b;
  }

  .pods-options-clear {
    border: 1px solid rgba(148, 163, 184, 0.45);
    background: rgba(248, 250, 252, 0.92);
    color: #475569;
    font-size: 0.56rem;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    padding: 0.2rem 0.35rem;
    line-height: 1;
    cursor: pointer;
  }

  .pods-options-clear:hover {
    border-color: rgba(134, 188, 37, 0.6);
    color: #a8d756;
  }

  .pods-option {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    width: 100%;
    background: transparent;
    border: 0;
    color: #334155;
    cursor: pointer;
    text-align: left;
    padding: 0.38rem 0.3rem;
    border-radius: 0;
    font-size: 0.7rem;
    line-height: 1.2;
    border-top: 1px solid rgba(148, 163, 184, 0.2);
  }

  .pods-option:first-of-type {
    border-top: 0;
  }

  .pods-option:hover {
    background: rgba(241, 245, 249, 0.95);
  }

  .pods-option-checkbox {
    width: 0.8rem;
    height: 0.8rem;
    border: 1px solid rgba(100, 116, 139, 0.5);
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .pods-option-checkbox.selected {
    border-color: rgba(134, 188, 37, 0.9);
    background: rgba(134, 188, 37, 0.2);
  }

  .pods-option-checkmark {
    color: #bef264;
    font-size: 0.64rem;
    line-height: 1;
    font-weight: 700;
  }

  .pods-option-label {
    flex: 1;
  }

  :global(.pods-input) {
    color: #334155;
  }

  @media (max-width: 900px) {
    .pods-filter-bar {
      justify-content: flex-start;
    }

    .pods-filter-controls {
      width: 100%;
      flex-wrap: wrap;
    }

    .pods-select-wrap,
    .pods-input-wrap {
      width: 100%;
    }
  }

  :global(.dark .pods-filter-trigger) {
    color: #d1d5db;
  }

  :global(.dark .pods-filter-chevron) {
    color: #94a3b8;
  }

  :global(.dark .pods-options-menu) {
    border: 1px solid rgba(255, 255, 255, 0.14);
    background: rgba(6, 8, 12, 0.98);
  }

  :global(.dark .pods-options-title) {
    color: #93a1b5;
  }

  :global(.dark .pods-options-clear) {
    border: 1px solid rgba(255, 255, 255, 0.2);
    background: rgba(255, 255, 255, 0.02);
    color: #d5deea;
  }

  :global(.dark .pods-option) {
    color: #d1d5db;
    border-top-color: rgba(255, 255, 255, 0.06);
  }

  :global(.dark .pods-option:hover) {
    background: rgba(15, 23, 42, 0.85);
  }

  :global(.dark .pods-option-checkbox) {
    border-color: rgba(148, 163, 184, 0.65);
  }

  :global(.dark .pods-option-checkbox.selected) {
    background: rgba(77, 124, 15, 0.24);
  }

  :global(.dark .pods-input) {
    color: #f3f4f6;
  }
</style>
