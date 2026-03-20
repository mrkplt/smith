<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { Badge, Button, Card, Label } from 'flowbite-svelte';
  import {
    AdjustmentsHorizontalOutline,
    BrainSolid,
    CheckCircleOutline,
    CogOutline,
    GithubSolid,
    GitlabSolid,
    GlobeOutline,
    PlusOutline
  } from 'flowbite-svelte-icons';
  import TopBar from '$lib/components/TopBar.svelte';
  import EmptyState from '$lib/components/EmptyState.svelte';
  import ProjectEditorDrawer from '$lib/components/ProjectEditorDrawer.svelte';
  import ProviderEditorDrawer from '$lib/components/ProviderEditorDrawer.svelte';
  import { loadChatSettings, resolveDefaultModel, resolveProviderType } from '$lib/chat/defaults';
  import { isChatEnabled, isIntegrationsEnabled, isProviderTypeEnabled, isSecretsEnabled } from '$lib/feature-flags';
  import { includeSelectedModel, loadProviderModels } from '$lib/providers/models';
  import { appState, pushToast } from '$lib/stores';
  import { apiBaseUrl, chatBaseUrl, deleteJSON, fetchJSON, postJSON, requestJSON } from '$lib/api';

  type SettingsSection = 'general' | 'providers' | 'projects' | 'chat' | 'integrations' | 'secrets';

  type SettingsNavItem = {
    id: SettingsSection;
    label: string;
    description: string;
  };

  const chatFeatureEnabled = $derived(isChatEnabled());
  const integrationsFeatureEnabled = $derived(isIntegrationsEnabled());
  const secretsFeatureEnabled = $derived(isSecretsEnabled());

  const sections = $derived.by((): SettingsNavItem[] => {
    const base: SettingsNavItem[] = [
      {
        id: 'general',
        label: 'General',
        description: 'Installation context and system defaults'
      },
      {
        id: 'providers',
        label: 'Providers',
        description: 'Reusable model and credential profiles'
      },
      {
        id: 'projects',
        label: 'Projects',
        description: 'Repository runtime contexts for loops'
      }
    ];
    if (chatFeatureEnabled) {
      base.push({
        id: 'chat',
        label: 'Chat',
        description: 'Operator assistant behavior and defaults'
      });
    }
    if (integrationsFeatureEnabled) {
      base.push({
        id: 'integrations',
        label: 'Integrations',
        description: 'External systems and repository connections'
      });
    }
    if (secretsFeatureEnabled) {
      base.push({
        id: 'secrets',
        label: 'Secrets',
        description: 'Credential references used by settings profiles'
      });
    }
    return base;
  });

  let providerProfileID = $state('');
  let defaultModel = $state('');
  let modelOptions = $state<string[]>([]);
  let modelOptionsBusy = $state(false);
  let thinkingLevel = $state('balanced');
  let providerApiKey = $state('');
  let preferFullScreen = $state(false);
  let providerProfiles = $state<any[]>([]);
  let secrets = $state<any[]>([]);
  let secretsBusy = $state(false);
  let secretID = $state('');
  let secretName = $state('');
  let secretDescription = $state('');
  let secretValue = $state('');
  let editingSecret = $state(false);

  let providerEditorOpen = $state(false);
  let selectedProvider = $state<any>(null);
  let projectEditorOpen = $state(false);
  let projectToEdit = $state<any>(null);
  let buildLabel = $state('local');

  const selectedChatProfile = $derived(
    providerProfiles.find((profile) => String(profile?.id || '') === providerProfileID) || null
  );
  const availableModelOptions = $derived(includeSelectedModel(modelOptions, defaultModel));

  const generalRows = $derived([
    { label: 'Installation', value: 'smith-console' },
    { label: 'Operator Identity', value: 'operator' },
    { label: 'Runtime Namespace', value: 'smith-system' },
    { label: 'API Base', value: apiBaseUrl },
    { label: 'Chat Base', value: chatBaseUrl },
    { label: 'Host', value: typeof window !== 'undefined' ? window.location.host : 'local' },
    { label: 'Build', value: buildLabel }
  ]);

  const activeSection = $derived(parseSection(page.url.searchParams.get('section')));

  onMount(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const config = (window as any).__SMITH_CONFIG__ || {};
    buildLabel = String(config.build || config.version || 'local');

    void loadProviderProfiles().then(loadChatSettingsFromBrowser);
    if (secretsFeatureEnabled) {
      void loadSecrets();
    }
    void refreshOnboardingState();
  });

  async function loadProviderProfiles() {
    try {
      const profiles = await fetchJSON('/v1/providers');
      providerProfiles = Array.isArray(profiles)
        ? profiles.filter((profile) => isProviderTypeEnabled(String(profile?.provider_type || profile?.id || '')))
        : [];
    } catch (err: any) {
      pushToast(err?.message || 'Failed to load provider profiles', 'err');
    }
  }

  function loadChatSettingsFromBrowser() {
    if (typeof window === 'undefined') {
      return;
    }
    const settings = loadChatSettings(window.localStorage, providerProfiles);
    providerProfileID = settings.providerProfileID;
    defaultModel = settings.defaultModel;
    thinkingLevel = settings.thinkingLevel;
    providerApiKey = settings.providerApiKey;
    preferFullScreen = settings.preferFullScreen;
    void loadModelOptions(providerProfileID);
  }

  async function loadModelOptions(nextProfileID: string) {
    const normalizedProfileID = String(nextProfileID || '').trim();
    const profile = providerProfiles.find((item) => String(item?.id || '').trim() === normalizedProfileID);
    const providerTypeHint = String(profile?.provider_type || '').trim();
    modelOptionsBusy = true;
    try {
      modelOptions = await loadProviderModels(normalizedProfileID, providerTypeHint);
    } finally {
      modelOptionsBusy = false;
    }
  }

  async function loadSecrets() {
    if (!secretsFeatureEnabled) {
      secrets = [];
      return;
    }
    try {
      const records = await fetchJSON('/v1/secrets');
      secrets = Array.isArray(records) ? records : [];
    } catch (err: any) {
      pushToast(err?.message || 'Failed to load secrets', 'err');
    }
  }

  async function refreshOnboardingState() {
    try {
      const readiness = await fetchJSON('/v1/onboarding/readiness');
      appState.update((state) => ({
        ...state,
        onboardingReady: !!readiness?.ready,
        onboardingChecked: true,
        onboardingState: readiness || null
      }));
    } catch {
      // Ignore readiness refresh failures in settings surfaces.
    }
  }

  function handleProviderSaved() {
    void loadProviderProfiles();
    if (secretsFeatureEnabled) {
      void loadSecrets();
    }
    void refreshOnboardingState();
  }

  function handleProjectSaved() {
    void refreshOnboardingState();
  }

  async function selectSection(section: SettingsSection) {
    const params = new URLSearchParams(page.url.searchParams);
    params.set('section', section);
    await goto(`/settings?${params.toString()}`, { keepFocus: true, noScroll: true });
  }

  function saveChatSettings() {
    if (typeof window === 'undefined') {
      return;
    }
    const normalizedProfileID = providerProfileID.trim();
    if (normalizedProfileID === '') {
      window.localStorage.removeItem('smith.chat.providerProfileID');
    } else {
      window.localStorage.setItem('smith.chat.providerProfileID', normalizedProfileID);
    }

    const resolvedProvider = resolveProviderType(providerProfiles, normalizedProfileID, '');
    if (resolvedProvider === '') {
      window.localStorage.removeItem('smith.chat.provider');
    } else {
      window.localStorage.setItem('smith.chat.provider', resolvedProvider);
    }

    if (defaultModel.trim() === '') {
      window.localStorage.removeItem('smith.chat.defaultModel');
    } else {
      window.localStorage.setItem('smith.chat.defaultModel', defaultModel.trim());
    }
    window.localStorage.setItem('smith.chat.thinkingLevel', thinkingLevel);
    if (providerApiKey.trim() === '') {
      window.localStorage.removeItem('smith.chat.providerApiKey');
    } else {
      window.localStorage.setItem('smith.chat.providerApiKey', providerApiKey.trim());
    }
    window.localStorage.setItem('smith.chat.preferFullScreen', String(preferFullScreen));
    pushToast('Chat settings saved.', 'ok');
  }

  function resetChatSettings() {
    if (providerProfiles.find((profile) => String(profile?.id || '') === 'codex-default')) {
      providerProfileID = 'codex-default';
    } else {
      providerProfileID = String(providerProfiles[0]?.id || '');
    }
    defaultModel = resolveDefaultModel(providerProfiles, providerProfileID, '');
    thinkingLevel = 'balanced';
    providerApiKey = '';
    preferFullScreen = false;
    saveChatSettings();
  }

  function selectChatProviderProfile(nextProfileID: string) {
    providerProfileID = nextProfileID;
    defaultModel = resolveDefaultModel(providerProfiles, nextProfileID, '');
    void loadModelOptions(nextProfileID);
  }

  function openConfig(providerProfile: any) {
    selectedProvider = providerProfile;
    providerEditorOpen = true;
  }

  function openNewProviderProfile() {
    selectedProvider = null;
    providerEditorOpen = true;
  }

  function openNewProject() {
    projectToEdit = null;
    projectEditorOpen = true;
  }

  function openEditProject(projectRecord: any) {
    projectToEdit = projectRecord;
    projectEditorOpen = true;
  }

  function getRepoIcon(url: string) {
    if (!url) return GlobeOutline;
    const lowerUrl = url.toLowerCase();
    if (lowerUrl.includes('github.com')) return GithubSolid;
    if (lowerUrl.includes('gitlab.com')) return GitlabSolid;
    return GlobeOutline;
  }

  function providerStatus(providerProfile: any): string {
    const secretRef = String(providerProfile?.secret_ref || '').trim();
    if (secretRef === '') {
      return 'needs secret';
    }
    return 'configured';
  }

  function providerSubtitle(providerProfile: any): string {
    const providerType = String(providerProfile?.provider_type || 'provider').toLowerCase();
    const defaultModel = String(providerProfile?.default_model || '').trim();
    if (defaultModel !== '') {
      return `${providerType} • ${defaultModel}`;
    }
    return providerType;
  }

  function parseSection(value: string | null): SettingsSection {
    if (value === 'chat' && !chatFeatureEnabled) {
      return 'general';
    }
    if (value === 'secrets' && !secretsFeatureEnabled) {
      return 'general';
    }
    if (value === 'integrations' && !integrationsFeatureEnabled) {
      return 'general';
    }
    if (value === 'providers' || value === 'projects' || value === 'chat' || value === 'general' || value === 'integrations' || value === 'secrets') {
      return value;
    }
    return 'general';
  }

  function startNewSecret() {
    editingSecret = false;
    secretID = '';
    secretName = '';
    secretDescription = '';
    secretValue = '';
  }

  function editSecret(record: any) {
    editingSecret = true;
    secretID = String(record?.id || '').trim();
    secretName = String(record?.name || '').trim();
    secretDescription = String(record?.description || '').trim();
    secretValue = '';
  }

  async function saveSecret() {
    if (secretID.trim() === '') {
      pushToast('Secret id is required', 'err');
      return;
    }
    if (!editingSecret && secretValue.trim() === '') {
      pushToast('Secret value is required', 'err');
      return;
    }
    secretsBusy = true;
    try {
      const payload = {
        id: secretID.trim(),
        name: secretName.trim(),
        description: secretDescription.trim(),
        value: secretValue.trim()
      };
      if (editingSecret) {
        await requestJSON(`/v1/secrets/${payload.id}`, 'PUT', payload);
      } else {
        await postJSON('/v1/secrets', payload);
      }
      pushToast(`Secret ${editingSecret ? 'updated' : 'created'} successfully`, 'ok');
      startNewSecret();
      await loadSecrets();
      await loadProviderProfiles();
    } catch (err: any) {
      pushToast(err?.message || 'Failed to save secret', 'err');
    } finally {
      secretsBusy = false;
    }
  }

  async function deleteSecret(secretRecord: any) {
    const id = String(secretRecord?.id || '').trim();
    if (id === '') {
      return;
    }
    if (!confirm(`Delete secret ${id}?`)) {
      return;
    }
    secretsBusy = true;
    try {
      await deleteJSON(`/v1/secrets/${id}`);
      pushToast('Secret deleted', 'ok');
      if (secretID === id) {
        startNewSecret();
      }
      await loadSecrets();
      await loadProviderProfiles();
    } catch (err: any) {
      pushToast(err?.message || 'Failed to delete secret', 'err');
    } finally {
      secretsBusy = false;
    }
  }
</script>

<TopBar title="Settings" />

<ProviderEditorDrawer
  bind:open={providerEditorOpen}
  onClose={() => providerEditorOpen = false}
  onSaved={handleProviderSaved}
  provider={selectedProvider}
  secretOptions={secrets}
/>
<ProjectEditorDrawer bind:open={projectEditorOpen} onClose={() => projectEditorOpen = false} onSaved={handleProjectSaved} {projectToEdit} />

<section class="px-4 pb-6">
  <div class="max-w-7xl border border-gray-800 bg-black/60 md:grid md:grid-cols-[260px_1fr]">
    <aside class="border-b border-gray-800 md:border-b-0 md:border-r md:border-gray-800 p-4 md:p-5">
      <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-gray-500 mb-3">Settings Menu</div>
      <nav class="space-y-3">
        {#each sections as section}
          <button
            class={`w-full text-left rounded-none border px-3 py-3 transition-colors ${activeSection === section.id ? 'border-[#86BC25] bg-[#86BC25]/10 text-white' : 'border-gray-800 text-gray-400 hover:text-white hover:border-gray-700'}`}
            onclick={() => selectSection(section.id)}
          >
            <div class="text-xs uppercase tracking-widest font-bold">{section.label}</div>
            <div class="text-[10px] mt-2 text-gray-500 leading-relaxed">{section.description}</div>
          </button>
        {/each}
      </nav>
    </aside>

    <div class="p-6 border-t border-gray-800 md:border-t-0">
      {#if activeSection === 'general'}
        <div class="pb-6 border-b border-gray-800">
          <h2 class="text-lg font-bold text-white uppercase tracking-tight">General</h2>
          <p class="mt-2 text-sm text-gray-400 max-w-2xl">
            Installation-level context and defaults for this Smith control plane.
          </p>
        </div>

        <div class="pt-6 grid grid-cols-1 md:grid-cols-2 gap-4 max-w-4xl">
          {#each generalRows as row}
            <Card class="bg-black border-gray-800 rounded-none p-0">
              <div class="p-4 space-y-2">
                <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-gray-500">{row.label}</div>
                <div class="text-sm text-gray-200 font-mono break-all">{row.value}</div>
              </div>
            </Card>
          {/each}
        </div>
      {:else if activeSection === 'chat'}
        <div class="pb-6 border-b border-gray-800">
          <h2 class="text-lg font-bold text-white uppercase tracking-tight">Chat Settings</h2>
          <p class="mt-2 text-sm text-gray-400">
            Configure operator chat defaults. Changes apply to new chat sessions in the drawer and full-screen assistant.
          </p>
        </div>

        <div class="pt-6 space-y-5 max-w-3xl">
          <div>
            <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Provider Profile</Label>
            <select
              class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
              value={providerProfileID}
              oninput={(event) => selectChatProviderProfile((event.currentTarget as HTMLSelectElement).value)}
            >
              {#if providerProfiles.length === 0}
                <option value="">No provider profiles available</option>
              {:else}
                {#each providerProfiles as providerProfile}
                  <option value={providerProfile.id}>{providerProfile.name || providerProfile.id}</option>
                {/each}
              {/if}
            </select>
            {#if selectedChatProfile}
              <p class="mt-2 text-[11px] text-gray-500">
                Uses provider type `{selectedChatProfile.provider_type || 'unknown'}` with secret reference `{selectedChatProfile.secret_ref || 'none'}`.
              </p>
            {/if}
          </div>

          <div>
            <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Default Model</Label>
            <select
              class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
              value={defaultModel}
              oninput={(event) => defaultModel = (event.currentTarget as HTMLSelectElement).value}
            >
              <option value="">Use provider profile default</option>
              {#if modelOptionsBusy}
                <option value={defaultModel} disabled>{defaultModel !== '' ? defaultModel : 'Loading models...'}</option>
              {/if}
              {#each availableModelOptions as model}
                <option value={model}>{model}</option>
              {/each}
            </select>
            {#if selectedChatProfile && selectedChatProfile.default_model}
              <p class="mt-2 text-[11px] text-gray-500">
                Profile default: `{selectedChatProfile.default_model}`
              </p>
            {/if}
          </div>

          <div>
            <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Thinking Level</Label>
            <select
              class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
              value={thinkingLevel}
              oninput={(event) => thinkingLevel = (event.currentTarget as HTMLSelectElement).value}
            >
              <option value="quick">Quick</option>
              <option value="balanced">Balanced</option>
              <option value="deep">Deep</option>
            </select>
            <p class="mt-2 text-[11px] text-gray-500">
              Thinking level maps to provider reasoning effort (`quick`, `balanced`, `deep`) without exposing raw model internals.
            </p>
          </div>

          <div>
            <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Provider API Key (Optional Override)</Label>
            <input
              type="password"
              class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
              placeholder="Leave blank to use cluster default runtime key"
              value={providerApiKey}
              oninput={(event) => providerApiKey = (event.currentTarget as HTMLInputElement).value}
            />
            <p class="mt-2 text-[11px] text-gray-500">
              Stored locally in this browser and attached only to chat session context when set.
            </p>
          </div>

          <label class="flex items-center gap-3 text-sm text-gray-300">
            <input
              type="checkbox"
              class="h-4 w-4 accent-[#86BC25]"
              checked={preferFullScreen}
              onchange={(event) => preferFullScreen = (event.currentTarget as HTMLInputElement).checked}
            />
            Open operator chat in full-screen assistant by default
          </label>

          <div class="pt-4 border-t border-gray-800 flex gap-3">
            <Button color="alternative" class="bg-[#86BC25] text-black font-bold rounded-none px-5" onclick={saveChatSettings}>
              Save Settings
            </Button>
            <Button color="alternative" class="rounded-none border-gray-700" onclick={resetChatSettings}>
              Reset to Defaults
            </Button>
          </div>
        </div>
      {:else if activeSection === 'providers'}
        <div class="pb-6 border-b border-gray-800 flex items-end justify-between gap-4">
          <div>
            <h2 class="text-lg font-bold text-white uppercase tracking-tight">Provider Profiles</h2>
            <p class="mt-2 text-sm text-gray-400 max-w-2xl">
              Providers are reusable model profiles referenced by chat, projects, and automation. Configure credentials and defaults here instead of treating them as runtime surfaces.
            </p>
          </div>
          <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7 rounded-none" onclick={openNewProviderProfile}>
            <PlusOutline size="xs" class="mr-1.5" />
            New Profile
          </Button>
        </div>

        {#if providerProfiles.length > 0}
          <Card class="mt-6 bg-black border-gray-800 rounded-none p-0">
            <div class="p-4 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-gray-500">Next Step</div>
                <div class="mt-1 text-sm text-gray-300">Provider setup is complete. Continue to project setup to unlock loop execution.</div>
              </div>
              <Button color="alternative" class="rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest" onclick={() => void selectSection('projects')}>
                Add Project
              </Button>
            </div>
          </Card>
        {/if}

        <div class="pt-6 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {#if providerProfiles.length === 0}
            <div class="col-span-full py-8">
              <EmptyState
                title="No Provider Profiles"
                description="Create provider profiles that can be reused by projects, chat sessions, and loop execution."
                buttonText="Create Profile"
                buttonHref="/settings?section=providers"
                icon="🧠"
              />
            </div>
          {:else}
            {#each providerProfiles as providerProfile}
              {@const providerConnection = providerStatus(providerProfile)}
              <Card class="bg-black border-gray-800 hover:border-[#86BC25]/50 transition-all p-0 overflow-hidden rounded-none">
                <div class="p-6 flex flex-col h-full gap-4">
                  <div class="flex justify-between items-start">
                    <div class="w-12 h-12 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
                      <BrainSolid size="lg" />
                    </div>
                    <Badge
                      color={providerConnection === 'needs secret' ? 'red' : 'green'}
                      class={`uppercase text-[10px] font-bold px-2 py-0.5 rounded-none ${providerConnection === 'needs secret' ? 'bg-red-900/40 text-red-200 border border-red-700/50' : 'bg-[#86BC25] text-black'}`}
                    >
                      {providerConnection}
                    </Badge>
                  </div>

                  <div>
                    <h3 class="text-xl font-bold text-white tracking-tight uppercase">{providerProfile.name || providerProfile.id}</h3>
                    <p class="text-sm text-gray-400 mt-1">{providerSubtitle(providerProfile)}</p>
                  </div>

                  <div class="text-sm text-gray-500 leading-relaxed font-medium">
                    ID: <span class="font-mono text-gray-400">{providerProfile.id}</span>
                  </div>

                  <div class="mt-auto pt-6 border-t border-gray-900 flex items-center justify-between">
                    <Button
                      color="alternative"
                      class="flex items-center bg-[#86BC25] text-black hover:bg-[#6b961d] font-bold uppercase text-[9px] tracking-widest px-6 py-2 rounded-none transition-all h-7"
                      onclick={() => openConfig(providerProfile)}
                    >
                      <AdjustmentsHorizontalOutline size="xs" class="mr-2" />
                      Configure
                    </Button>
                    <div class="text-[#86BC25]">
                      <CheckCircleOutline size="lg" />
                    </div>
                  </div>
                </div>
              </Card>
            {/each}
          {/if}
        </div>
      {:else if activeSection === 'projects'}
        <div class="pb-6 border-b border-gray-800 flex items-end justify-between gap-4">
          <div>
            <h2 class="text-lg font-bold text-white uppercase tracking-tight">Projects</h2>
            <p class="mt-2 text-sm text-gray-400 max-w-2xl">
              Projects define repository-based runtime contexts for loops. Manage them in Settings so runtime views stay focused on active execution.
            </p>
          </div>
          <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7 rounded-none" onclick={openNewProject}>
            <PlusOutline size="xs" class="mr-1.5" />
            New Project
          </Button>
        </div>

        <div class="pt-6 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {#if $appState.projects.length === 0}
            <div class="col-span-full py-12">
              <EmptyState
                title="No Projects Configured"
                description="Projects define the repositories and environments for your autonomous loops. Create one to get started."
                buttonText="Create Project"
                buttonHref="/settings?section=projects"
                icon="🏗️"
              />
            </div>
          {:else}
            {#each $appState.projects as projectRecord (projectRecord.id)}
              {@const Icon = getRepoIcon(projectRecord.repo_url)}
              <Card class="bg-black border-gray-800 hover:border-[#86BC25]/50 transition-all p-6 rounded-none">
                <div class="flex flex-col h-full gap-4">
                  <div class="flex justify-between items-start gap-2">
                    <div class="flex items-center gap-3">
                      <div class="w-10 h-10 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
                        <Icon size="lg" />
                      </div>
                      <div>
                        <h3 class="text-lg font-bold text-white tracking-tight uppercase">{projectRecord.name}</h3>
                        <code class="text-[10px] text-gray-500 font-mono">{projectRecord.id}</code>
                      </div>
                    </div>
                    <Button color="alternative" size="xs" class="p-2 border-gray-700 hover:bg-slate-800 rounded-none h-8 w-8" onclick={() => openEditProject(projectRecord)}>
                      <CogOutline size="sm" />
                    </Button>
                  </div>

                  <div class="space-y-2">
                    <div class="text-[10px] font-bold uppercase tracking-widest text-gray-500">Repository</div>
                    <div class="text-sm text-gray-300 font-mono truncate bg-black p-2 border border-gray-800">
                      {projectRecord.repo_url}
                    </div>
                  </div>

                  <div class="mt-auto pt-4 border-t border-gray-900 flex justify-between items-center">
                    <div class="flex gap-2">
                      <Badge color="gray" class="bg-slate-800 text-gray-400 text-[10px] rounded-none">{projectRecord.github_user || 'no-user'}</Badge>
                      <Badge color="gray" class="bg-slate-800 text-gray-400 text-[10px] rounded-none">provider: {projectRecord.provider_profile_id || 'codex-default'}</Badge>
                    </div>
                    <div class="text-[10px] font-mono text-gray-600 uppercase font-bold tracking-widest">
                      RT: {projectRecord.runtime_image ? 'custom' : 'default'}
                    </div>
                  </div>
                </div>
              </Card>
            {/each}
          {/if}
        </div>
      {:else if activeSection === 'integrations'}
        <div class="pb-6 border-b border-gray-800">
          <h2 class="text-lg font-bold text-white uppercase tracking-tight">Integrations</h2>
          <p class="mt-2 text-sm text-gray-400 max-w-2xl">
            External integrations will be managed here, starting with GitHub App connection and repository authorization.
          </p>
        </div>

        <div class="pt-6 max-w-3xl">
          <Card class="bg-black border-gray-800 rounded-none p-0">
            <div class="p-6">
              <div class="text-xs uppercase tracking-[0.2em] font-bold text-gray-500">Coming Next</div>
              <div class="mt-3 text-sm text-gray-300 leading-relaxed">
                This section will expose GitHub installation status, connected integrations, and repository sync actions.
              </div>
            </div>
          </Card>
        </div>
      {:else if activeSection === 'secrets'}
        <div class="pb-6 border-b border-gray-800">
          <h2 class="text-lg font-bold text-white uppercase tracking-tight">Secrets</h2>
          <p class="mt-2 text-sm text-gray-400 max-w-2xl">
            Manage reusable secret references for provider profiles and integrations. Secret values are write-only and never returned.
          </p>
        </div>

        <div class="pt-6 grid grid-cols-1 xl:grid-cols-[360px_1fr] gap-6">
          <Card class="bg-black border-gray-800 rounded-none p-0 h-fit">
            <div class="p-6 space-y-4">
              <div class="flex items-center justify-between">
                <div class="text-xs uppercase tracking-[0.2em] font-bold text-gray-500">
                  {editingSecret ? 'Rotate / Update Secret' : 'Create Secret'}
                </div>
                {#if editingSecret}
                  <Button color="alternative" size="xs" class="rounded-none border-gray-700 bg-slate-900 text-gray-300" onclick={startNewSecret} disabled={secretsBusy}>New</Button>
                {/if}
              </div>

              <div>
                <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Secret ID</Label>
                <input
                  data-testid="settings-secret-id"
                  class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2 font-mono"
                  value={secretID}
                  oninput={(event) => secretID = (event.currentTarget as HTMLInputElement).value}
                  disabled={secretsBusy || editingSecret}
                  placeholder="openai-key"
                />
              </div>

              <div>
                <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Display Name</Label>
                <input
                  data-testid="settings-secret-name"
                  class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
                  value={secretName}
                  oninput={(event) => secretName = (event.currentTarget as HTMLInputElement).value}
                  disabled={secretsBusy}
                  placeholder="OpenAI key"
                />
              </div>

              <div>
                <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Description</Label>
                <input
                  data-testid="settings-secret-description"
                  class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
                  value={secretDescription}
                  oninput={(event) => secretDescription = (event.currentTarget as HTMLInputElement).value}
                  disabled={secretsBusy}
                  placeholder="Primary credential for OpenAI provider"
                />
              </div>

              <div>
                <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">{editingSecret ? 'New Value (Optional)' : 'Secret Value'}</Label>
                <input
                  data-testid="settings-secret-value"
                  type="password"
                  class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
                  value={secretValue}
                  oninput={(event) => secretValue = (event.currentTarget as HTMLInputElement).value}
                  disabled={secretsBusy}
                  placeholder={editingSecret ? 'Leave empty to keep current value' : 'sk-...'}
                />
              </div>

              <div class="pt-2 flex gap-3 justify-end">
                <Button color="alternative" size="sm" class="rounded-none border-gray-700 bg-slate-900 text-gray-300" onclick={startNewSecret} disabled={secretsBusy}>Reset</Button>
                <Button color="alternative" size="sm" data-testid="settings-secret-save" class="rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest px-5" onclick={saveSecret} disabled={secretsBusy}>
                  {editingSecret ? 'Update Secret' : 'Create Secret'}
                </Button>
              </div>
            </div>
          </Card>

          <div class="space-y-4">
            {#if secrets.length === 0}
              <EmptyState
                title="No secrets configured"
                description="Create your first secret reference, then select it from provider profiles."
              />
            {:else}
              {#each secrets as secretRecord}
                <Card class="bg-black border-gray-800 rounded-none p-0">
                  <div class="p-5 flex flex-col gap-3">
                    <div class="flex items-start justify-between gap-3">
                      <div>
                        <div class="text-sm font-bold text-white font-mono">{secretRecord.id}</div>
                        <div class="text-xs uppercase tracking-widest text-gray-500 mt-1">{secretRecord.name || secretRecord.id}</div>
                      </div>
                      <div class="flex gap-2">
                        <Button color="alternative" size="xs" class="p-2 border-gray-700 hover:bg-slate-800 rounded-none h-8 w-8" aria-label={`Edit secret ${secretRecord.id}`} onclick={() => editSecret(secretRecord)}>
                          <CogOutline size="sm" />
                        </Button>
                        <Button color="alternative" size="xs" class="p-2 border-gray-700 hover:bg-red-900/40 rounded-none h-8 w-8" aria-label={`Delete secret ${secretRecord.id}`} onclick={() => deleteSecret(secretRecord)}>
                          ×
                        </Button>
                      </div>
                    </div>

                    {#if secretRecord.description}
                      <div class="text-sm text-gray-300 leading-relaxed">{secretRecord.description}</div>
                    {/if}

                    <div class="flex flex-wrap gap-2 pt-2 border-t border-gray-900">
                      <Badge color="gray" class="bg-slate-800 text-gray-400 text-[10px] rounded-none">
                        {secretRecord.has_value ? 'value set' : 'no value'}
                      </Badge>
                      {#if secretRecord.value_masked}
                        <Badge color="gray" class="bg-slate-800 text-gray-400 text-[10px] rounded-none font-mono">{secretRecord.value_masked}</Badge>
                      {/if}
                    </div>
                  </div>
                </Card>
              {/each}
            {/if}
          </div>
        </div>
      {/if}
    </div>
  </div>
</section>
