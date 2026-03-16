<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import TopBar from '$lib/components/TopBar.svelte';

  onMount(() => {
    void goto('/settings?section=projects', { replaceState: true });
  });
</script>

<TopBar title="Projects" />

<section class="px-4 py-10">
  <div class="max-w-3xl border border-gray-800 bg-black/60 p-6">
    <h2 class="text-lg font-bold text-white uppercase tracking-tight">Projects Moved</h2>
    <p class="mt-3 text-sm text-gray-400">
      Project configuration now lives under Settings.
      If redirect does not happen automatically, open
      <a href="/settings?section=projects" class="text-[#86BC25] hover:underline">Settings → Projects</a>.
    </p>
  </div>
</section>
