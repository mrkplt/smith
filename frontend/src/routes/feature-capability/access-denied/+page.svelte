<script lang="ts">
  import TopBar from '$lib/components/TopBar.svelte';
</script>

<TopBar title="Feature Capability" />

<section class="px-4 pb-8">
  <div class="max-w-3xl border border-red-900/80 bg-red-950/20 p-6 md:p-8">
    <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-red-300">Access Denied</div>
    <h2 class="mt-3 text-xl font-bold text-white uppercase tracking-tight">You do not have access to this feature flow</h2>
    <p class="mt-3 text-sm text-gray-300">
      Your account is missing the required permission: <code>feature_capability:access</code>.
      Contact an administrator if you need this capability.
    </p>
  </div>
</section>
