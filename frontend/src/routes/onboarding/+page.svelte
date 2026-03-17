<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Badge, Button, Card } from 'flowbite-svelte';
  import TopBar from '$lib/components/TopBar.svelte';
  import { fetchJSON } from '$lib/api';
  import { appState, pushToast } from '$lib/stores';

  let loading = $state(true);
  let readiness = $state<any>(null);

  const requirements = $derived(Array.isArray(readiness?.requirements) ? readiness.requirements : []);
  const providerRequirement = $derived(requirements.find((item: any) => item?.id === 'provider_catalog') || null);
  const providerReady = $derived(String(providerRequirement?.status || '').toLowerCase() === 'complete');

  onMount(() => {
    void refreshReadiness();
  });

  async function refreshReadiness() {
    loading = true;
    try {
      const response = await fetchJSON('/v1/onboarding/readiness');
      readiness = response || null;
      appState.update((state) => ({
        ...state,
        onboardingReady: !!response?.ready,
        onboardingChecked: true,
        onboardingState: response || null
      }));
    } catch (err: any) {
      pushToast(err?.message || 'Failed to load onboarding readiness', 'err');
    } finally {
      loading = false;
    }
  }
</script>

<TopBar title="Onboarding" />

<section class="px-4 pb-8">
  <div class="max-w-5xl mx-auto border border-gray-800 bg-black/60 p-6 space-y-6">
    <div>
      <h2 class="text-xl font-bold text-white uppercase tracking-tight">Workspace Setup</h2>
      <p class="mt-2 text-sm text-gray-400 max-w-3xl">
        Complete setup before runtime surfaces are enabled. Sequence: configure a supported provider first, then add a project/repository.
      </p>
    </div>

    {#if loading}
      <Card class="bg-black border-gray-800 rounded-none p-4">
        <p class="text-sm text-gray-400">Checking onboarding readiness...</p>
      </Card>
    {:else if readiness?.ready}
      <Card class="bg-black border-gray-800 rounded-none p-4">
        <div class="flex items-center justify-between gap-4">
          <div>
            <div class="text-sm font-bold text-[#86BC25] uppercase tracking-widest">Setup complete</div>
            <div class="mt-1 text-sm text-gray-300">Provider and project requirements are satisfied for this workspace.</div>
          </div>
          <Button color="alternative" class="rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest" onclick={() => goto('/pods')}>
            Open Pods
          </Button>
        </div>
      </Card>
    {:else}
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card class="bg-black border-gray-800 rounded-none p-0">
          <div class="p-5 space-y-3">
            <div class="text-xs uppercase tracking-[0.2em] text-gray-500 font-bold">Requirements</div>
            {#if requirements.length === 0}
              <p class="text-sm text-gray-400">No readiness data available.</p>
            {:else}
              {#each requirements as requirement}
                {@const complete = String(requirement?.status || '').toLowerCase() === 'complete'}
                <div class="border border-gray-800 px-3 py-3 flex items-start justify-between gap-3">
                  <div>
                    <div class="text-sm text-white font-semibold">{requirement?.label || requirement?.id}</div>
                    {#if requirement?.details}
                      <div class="text-xs text-gray-500 mt-1">{requirement.details}</div>
                    {/if}
                  </div>
                  <Badge class={`rounded-none uppercase text-[10px] ${complete ? 'bg-[#86BC25] text-black' : 'bg-slate-800 text-gray-300'}`}>
                    {complete ? 'complete' : 'missing'}
                  </Badge>
                </div>
              {/each}
            {/if}
          </div>
        </Card>

        <Card class="bg-black border-gray-800 rounded-none p-0">
          <div class="p-5 space-y-4">
            <div class="text-xs uppercase tracking-[0.2em] text-gray-500 font-bold">Next Steps</div>
            <p class="text-sm text-gray-300">
              {#if !providerReady}
                Configure a supported provider profile before project/repository setup.
              {:else}
                Add your project repository configuration to enable loop execution.
              {/if}
            </p>

            <div class="flex flex-wrap gap-3">
              <Button color="alternative" class="rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest" onclick={() => goto('/settings?section=providers')}>
                Configure Provider
              </Button>
              <Button color="alternative" class="rounded-none border-gray-700 bg-slate-900 text-gray-300 uppercase text-[10px] tracking-widest" onclick={() => goto('/settings?section=projects')} disabled={!providerReady}>
                Add Project
              </Button>
              <Button color="alternative" class="rounded-none border-gray-700 bg-slate-900 text-gray-300 uppercase text-[10px] tracking-widest" onclick={refreshReadiness}>
                Refresh Status
              </Button>
            </div>

            {#if !providerReady}
              <p class="text-[11px] text-gray-500">Project setup is disabled until at least one provider profile is configured.</p>
            {/if}
          </div>
        </Card>
      </div>
    {/if}
  </div>
</section>
