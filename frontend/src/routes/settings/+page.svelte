<script lang="ts">
  import { onMount } from 'svelte';
  import { Button, Label } from 'flowbite-svelte';
  import TopBar from '$lib/components/TopBar.svelte';
  import { pushToast } from '$lib/stores';

  type SettingsSection = 'chat';

  let provider = $state('');
  let thinkingLevel = $state('balanced');
  let providerApiKey = $state('');
  let preferFullScreen = $state(false);
  let activeSection = $state<SettingsSection>('chat');

  const sections: { id: SettingsSection; label: string; description: string }[] = [
    {
      id: 'chat',
      label: 'Chat',
      description: 'Operator assistant behavior and defaults'
    }
  ];

  onMount(() => {
    if (typeof window === 'undefined') {
      return;
    }
    provider = window.localStorage.getItem('smith.chat.provider') || '';
    const savedThinking = window.localStorage.getItem('smith.chat.thinkingLevel');
    if (savedThinking === 'quick' || savedThinking === 'balanced' || savedThinking === 'deep') {
      thinkingLevel = savedThinking;
    }
    providerApiKey = window.localStorage.getItem('smith.chat.providerApiKey') || '';
    preferFullScreen = window.localStorage.getItem('smith.chat.preferFullScreen') === 'true';
  });

  function saveSettings() {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem('smith.chat.provider', provider.trim());
    window.localStorage.setItem('smith.chat.thinkingLevel', thinkingLevel);
    if (providerApiKey.trim() === '') {
      window.localStorage.removeItem('smith.chat.providerApiKey');
    } else {
      window.localStorage.setItem('smith.chat.providerApiKey', providerApiKey.trim());
    }
    window.localStorage.setItem('smith.chat.preferFullScreen', String(preferFullScreen));
    pushToast('Chat settings saved.', 'ok');
  }

  function resetSettings() {
    provider = '';
    thinkingLevel = 'balanced';
    providerApiKey = '';
    preferFullScreen = false;
    saveSettings();
  }
</script>

<TopBar title="Settings" />

<section class="px-4 pb-6">
  <div class="max-w-6xl border border-gray-800 bg-black/60 md:grid md:grid-cols-[240px_1fr]">
    <aside class="border-b border-gray-800 md:border-b-0 md:border-r md:border-gray-800 p-4 md:p-5">
      <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-gray-500 mb-3">Settings Menu</div>
      <nav class="space-y-3">
        {#each sections as section}
          <button
            class="w-full text-left rounded-none border px-3 py-2 transition-colors {activeSection === section.id ? 'border-[#86BC25] bg-[#86BC25]/10 text-white' : 'border-gray-800 text-gray-400 hover:text-white hover:border-gray-700'}"
            onclick={() => activeSection = section.id}
          >
            <div class="text-xs uppercase tracking-widest font-bold">{section.label}</div>
            <div class="text-[10px] mt-1 text-gray-500">{section.description}</div>
          </button>
        {/each}
      </nav>
    </aside>

    <div class="p-6 border-t border-gray-800 md:border-t-0">
      {#if activeSection === 'chat'}
        <div class="pb-6 border-b border-gray-800">
          <h2 class="text-lg font-bold text-white uppercase tracking-tight">Chat Defaults</h2>
          <p class="mt-2 text-sm text-gray-400">
            Configure operator chat defaults. Changes apply to new chat sessions in drawer and full-screen assistant.
          </p>
        </div>

        <div class="pt-6 space-y-5">
          <div>
            <Label class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Preferred Provider</Label>
            <select
              class="w-full bg-slate-900 border border-gray-800 text-white text-sm rounded-none px-3 py-2"
              value={provider}
              oninput={(event) => provider = (event.currentTarget as HTMLSelectElement).value}
            >
              <option value="">Service default</option>
              <option value="openai">openai</option>
              <option value="anthropic">anthropic</option>
              <option value="google">google</option>
            </select>
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
            {#if provider === '' || provider === 'openai'}
              <p class="mt-2 text-[11px] text-gray-500">
                OpenAI currently uses `gpt-5.4` by default; thinking level should tune reasoning effort rather than swap models.
              </p>
            {/if}
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
            <Button color="alternative" class="bg-[#86BC25] text-black font-bold rounded-none px-5" onclick={saveSettings}>
              Save Settings
            </Button>
            <Button color="alternative" class="rounded-none border-gray-700" onclick={resetSettings}>
              Reset to Defaults
            </Button>
          </div>
        </div>
      {/if}
    </div>
  </div>
</section>
