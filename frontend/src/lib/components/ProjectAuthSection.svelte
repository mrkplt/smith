<script lang="ts">
  import { Input, Label } from 'flowbite-svelte';

  interface Props {
    githubUser: string;
    githubCredential: string;
    busy: boolean;
    onGitHubUserChange: (value: string) => void;
    onGitHubCredentialChange: (value: string) => void;
  }

  let { githubUser, githubCredential, busy, onGitHubUserChange, onGitHubCredentialChange }: Props = $props();
</script>

<div class="space-y-6">
  <h4 class="text-[10px] font-bold uppercase tracking-[0.3em] text-[#86BC25]">Authentication</h4>
  <div class="grid grid-cols-2 gap-4">
    <div>
      <Label for="user" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Git User</Label>
      <Input
        type="text"
        id="user"
        placeholder="username"
        value={githubUser}
        oninput={(event) => onGitHubUserChange((event.currentTarget as HTMLInputElement).value)}
        disabled={busy}
        class="bg-black border-gray-800 text-white font-bold rounded-none focus:border-[#86BC25]"
      />
    </div>
    <div>
      <Label for="token" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Secret Token</Label>
      <Input
        type="password"
        id="token"
        placeholder="••••••••"
        value={githubCredential}
        oninput={(event) => onGitHubCredentialChange((event.currentTarget as HTMLInputElement).value)}
        disabled={busy}
        class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]"
      />
    </div>
  </div>
</div>
