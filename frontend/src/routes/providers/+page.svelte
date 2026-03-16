<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import TopBar from '$lib/components/TopBar.svelte';

  onMount(() => {
    void goto('/settings?section=providers', { replaceState: true });
  });
</script>

<TopBar title="Providers" />

<section class="px-4 py-10">
  <div class="max-w-3xl border border-gray-800 bg-black/60 p-6">
    <h2 class="text-lg font-bold text-white uppercase tracking-tight">Providers Moved</h2>
    <p class="mt-3 text-sm text-gray-400">
      Provider profiles now live under Settings.
      If redirect does not happen automatically, open
      <a href="/settings?section=providers" class="text-[#86BC25] hover:underline">Settings → Providers</a>.
    </p>
  </div>
</section>
