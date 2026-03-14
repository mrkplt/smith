<script lang="ts">
  import { Input, Label, Textarea } from 'flowbite-svelte';

  interface Props {
    method: string;
    loopName: string;
    branch: string;
    prompt: string;
    onLoopNameChange: (value: string) => void;
    onBranchChange: (value: string) => void;
    onPromptChange: (value: string) => void;
  }

  let { method, loopName, branch, prompt, onLoopNameChange, onBranchChange, onPromptChange }: Props = $props();
</script>

<div class="space-y-4">
  <div>
    <Label for="loop-name" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Loop Identifier</Label>
    <Input
      type="text"
      id="loop-name"
      placeholder="fix-authentication-bug"
      value={loopName}
      oninput={(event) => onLoopNameChange((event.currentTarget as HTMLInputElement).value)}
      class="bg-slate-900 border-gray-800 text-white rounded-none"
    />
  </div>
  <div>
    <Label for="branch" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Branch Name</Label>
    <Input
      type="text"
      id="branch"
      placeholder="feature/auth-fix"
      value={branch}
      oninput={(event) => onBranchChange((event.currentTarget as HTMLInputElement).value)}
      class="bg-slate-900 border-gray-800 text-white rounded-none"
    />
  </div>
  {#if method === 'load_prd'}
    <div>
      <Label for="prompt" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">PRD Content / Prompt</Label>
      <Textarea
        id="prompt"
        rows={6}
        placeholder="Paste JSON or instructions..."
        value={prompt}
        oninput={(event) => onPromptChange((event.currentTarget as HTMLTextAreaElement).value)}
        class="bg-slate-900 border-gray-800 text-white rounded-none"
      />
    </div>
  {/if}
</div>
