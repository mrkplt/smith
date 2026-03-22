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

  <Button color="alternative" class="smith-btn" onclick={triggerUpload}>
    <CloudArrowUpOutline size="xs" class="mr-1.5" />
    Upload PRD
  </Button>

  <Button color="alternative" class="smith-btn smith-btn-primary" onclick={onCreateNew}>
    <PlusOutline size="xs" class="mr-1.5" />
    New Doc
  </Button>
</div>
