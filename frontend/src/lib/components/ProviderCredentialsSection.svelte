<script lang="ts">
  import { Helper, Input, Label } from 'flowbite-svelte';

  interface Props {
    apiKey: string;
    accountId: string;
    busy: boolean;
    onAPIKeyChange: (value: string) => void;
    onAccountIDChange: (value: string) => void;
  }

  let { apiKey, accountId, busy, onAPIKeyChange, onAccountIDChange }: Props = $props();
</script>

<div class="space-y-6">
  <div>
    <Label for="api-key" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">OpenAI API Key</Label>
    <Input
      type="password"
      id="api-key"
      placeholder="sk-..."
      value={apiKey}
      oninput={(event) => onAPIKeyChange((event.currentTarget as HTMLInputElement).value)}
      disabled={busy}
      required
      class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]"
    />
    <Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">Paste your secret key from the OpenAI dashboard.</Helper>
  </div>

  <div>
    <Label for="account-id" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Account ID (Optional)</Label>
    <Input
      type="text"
      id="account-id"
      placeholder="Optional Organization ID"
      value={accountId}
      oninput={(event) => onAccountIDChange((event.currentTarget as HTMLInputElement).value)}
      disabled={busy}
      class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]"
    />
  </div>
</div>
