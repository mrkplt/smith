<script lang="ts">
  import { Helper, Input, Label } from 'flowbite-svelte';
  import { GlobeOutline } from 'flowbite-svelte-icons';

  interface Props {
    name: string;
    repoUrl: string;
    busy: boolean;
    onNameChange: (value: string) => void;
    onRepoUrlChange: (value: string) => void;
  }

  let { name, repoUrl, busy, onNameChange, onRepoUrlChange }: Props = $props();
</script>

<div class="space-y-6">
  <div>
    <Label for="name" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Project Name</Label>
    <Input
      type="text"
      id="name"
      placeholder="Project Alpha"
      value={name}
      oninput={(event) => onNameChange((event.currentTarget as HTMLInputElement).value)}
      disabled={busy}
      required
      class="bg-black border-gray-800 text-white font-bold rounded-none focus:border-[#86BC25]"
    />
    <Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">This will be used to generate the system ID.</Helper>
  </div>

  <div>
    <Label for="repo" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Repository URL</Label>
    <div class="flex">
      <span class="inline-flex items-center px-3 bg-slate-900 border border-r-0 border-gray-800 text-gray-500">
        <GlobeOutline size="sm" />
      </span>
      <Input
        type="url"
        id="repo"
        placeholder="https://github.com/org/repo.git"
        value={repoUrl}
        oninput={(event) => onRepoUrlChange((event.currentTarget as HTMLInputElement).value)}
        disabled={busy}
        required
        class="bg-black border-gray-800 text-white font-mono text-sm rounded-none focus:border-[#86BC25]"
      />
    </div>
  </div>
</div>
