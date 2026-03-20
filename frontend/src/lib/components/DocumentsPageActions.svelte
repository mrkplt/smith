<script lang="ts">
  import { Button } from 'flowbite-svelte';
  import { CloudArrowUpOutline, PlusOutline } from 'flowbite-svelte-icons';

  interface Props {
    onCreateNew: () => void;
    onImportPRD: (file: File) => void;
  }

  let { onCreateNew, onImportPRD }: Props = $props();
  let fileInput: HTMLInputElement | null = null;

  function triggerUpload() {
    fileInput?.click();
  }

  function handleFileSelection(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }
    onImportPRD(file);
    input.value = '';
  }
</script>

<div class="flex items-center gap-2">
  <input
    bind:this={fileInput}
    type="file"
    class="hidden"
    accept=".md,.markdown,.json,text/markdown,application/json,text/plain"
    onchange={handleFileSelection}
  />

  <Button color="alternative" class="bg-black border-gray-800 text-[#86BC25] hover:bg-white/5 rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={triggerUpload}>
    <CloudArrowUpOutline size="xs" class="mr-1.5" />
    Upload PRD
  </Button>

  <Button color="alternative" class="bg-[#86BC25] text-black rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={onCreateNew}>
    <PlusOutline size="xs" class="mr-1.5" />
    New Doc
  </Button>
</div>
