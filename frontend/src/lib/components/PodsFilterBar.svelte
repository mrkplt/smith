<script lang="ts">
  import { Input, Select } from 'flowbite-svelte';

  interface Props {
    stateFilter: string;
    searchQuery: string;
    onStateFilterChange: (value: string) => void;
    onSearchQueryChange: (value: string) => void;
  }

  let { stateFilter, searchQuery, onStateFilterChange, onSearchQueryChange }: Props = $props();
</script>

<div class="flex items-center justify-start gap-4 -mt-14 mb-8 relative z-50 px-4 ml-auto w-fit">
  <div class="flex items-center gap-2">
    <div class="w-32">
      <Select
        value={stateFilter}
        onchange={(event) => onStateFilterChange((event.currentTarget as HTMLSelectElement).value)}
        size="sm"
        class="bg-black border-gray-800 text-gray-400 text-[10px] uppercase font-bold rounded-none h-7"
      >
        <option value="healthy">Healthy (Run + Done)</option>
        <option value="all">All States</option>
        <option value="active">Active Only</option>
        <option value="unresolved">Unresolved</option>
        <option value="running">Running</option>
        <option value="synced">Synced</option>
        <option value="flatline">Flatline</option>
        <option value="cancelled">Cancelled</option>
      </Select>
    </div>

    <div class="w-48">
      <Input
        type="search"
        placeholder="Filter ID..."
        value={searchQuery}
        oninput={(event) => onSearchQueryChange((event.currentTarget as HTMLInputElement).value)}
        size="sm"
        class="bg-black border-gray-800 text-white text-[10px] uppercase font-bold rounded-none h-7 px-3"
      />
    </div>
  </div>
</div>
