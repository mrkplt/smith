<script lang="ts">
	import { onMount } from 'svelte';
	import { appState } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';
	import PodTile from '$lib/components/PodTile.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import PodsFilterBar from '$lib/components/PodsFilterBar.svelte';
	import PodsStatsStrip from '$lib/components/PodsStatsStrip.svelte';
  import { GridOutline } from 'flowbite-svelte-icons';

	let stateFilter = $state('all');
	let searchQuery = $state('');

	function normalizeLoop(item: any) {
		const record = item.record || item.Record || {};
		const loopID = record.loop_id || record.LoopID || item.loop_id || "unknown-loop";
		const status = (record.state || record.State || "unknown").toLowerCase();
		const attempt = Number(record.attempt || record.Attempt || 0);
		const reason = record.reason || record.Reason || "";
		const revision = Number(item.revision || item.Revision || record.observed_revision || 0);
		return {
			loopID,
			project: record.project_id || record.project || record.project_name || "default",
			status,
			attempt,
			reason,
			revision,
		};
	}

	onMount(() => {
		// Data is populated by the global stream in layout
	});

	const filteredLoops = $derived(
		$appState.loops.filter((loop: any) => {
			const matchesState =
				stateFilter === "all" ||
				(stateFilter === "active" && (loop.status === "unresolved" || loop.status === "running")) ||
				loop.status === stateFilter;
			const query = searchQuery.toLowerCase();
			const matchesSearch = !searchQuery || String(loop.loopID).toLowerCase().includes(query) || String(loop.displayTitle || '').toLowerCase().includes(query);
			return matchesState && matchesSearch;
		})
	);

	const stats = $derived({
		total: $appState.loops.length,
		active: $appState.loops.filter((l: any) => l.status === "unresolved" || l.status === "running").length,
		flatline: $appState.loops.filter((l: any) => l.status === "flatline").length
	});

	const missingOnboardingRequirements = $derived(
		Array.isArray($appState.onboardingState?.missing) ? $appState.onboardingState.missing : []
	);
	const providerSetupMissing = $derived(
		missingOnboardingRequirements.includes('provider_catalog') ||
		missingOnboardingRequirements.includes('provider_binding')
	);

	function selectLoop(id: string) {
		appState.update(s => ({ ...s, selectedLoop: id }));
	}
</script>

<TopBar title="Pods" />

<PodsFilterBar
  {stateFilter}
  {searchQuery}
  onStateFilterChange={(value) => stateFilter = value}
  onSearchQueryChange={(value) => searchQuery = value}
/>

<PodsStatsStrip {stats} />

<section class="tiles-shell px-4">
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4" role="list">
    {#if $appState.onboardingChecked && !$appState.onboardingReady}
		<div class="col-span-full py-12">
			<EmptyState
				title={providerSetupMissing ? 'Provider Setup Required' : 'Setup Required'}
				description={providerSetupMissing
					? 'This dashboard is gated until at least one provider profile is configured. Add a provider to continue creating and attaching loops.'
					: 'This dashboard is gated until onboarding is complete. Configure a provider first, then add a project repository.'}
				buttonText={providerSetupMissing ? 'Open Providers' : 'Open Onboarding'}
				buttonHref={providerSetupMissing ? '/providers' : '/onboarding'}
				icon="🧭"
			/>
		</div>
    {:else if $appState.projects.length === 0}
      <div class="col-span-full py-12">
        <EmptyState 
          title="Welcome to SMITH" 
          description="To get started, you'll need to configure a project. Projects connect your repositories and enable autonomous development loops."
          buttonText="Configure Project"
          buttonHref="/projects"
          icon="🚀"
        />
      </div>
    {:else}
      {#each filteredLoops as loop (loop.loopID)}
        <PodTile 
          {loop} 
          selected={$appState.selectedLoop === loop.loopID} 
          onSelect={selectLoop} 
        />
      {:else}
        <div class="col-span-full py-20 bg-slate-900/10 border border-dashed border-gray-900 rounded-none flex flex-col items-center justify-center text-gray-600">
          <GridOutline size="xl" class="mb-4 opacity-20" />
          <p class="text-sm uppercase font-bold tracking-[0.2em]">No pods found matching filters.</p>
        </div>
      {/each}
    {/if}
  </div>
</section>
