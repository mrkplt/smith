<script lang="ts">
  import { Button, Checkbox } from 'flowbite-svelte';
  import { MessagesOutline, PlusOutline } from 'flowbite-svelte-icons';

  interface Props {
    showAll: boolean;
    onShowAllChange: (value: boolean) => void;
    onOpenChat: () => void;
    onCreateNew: () => void;
  }

  let { showAll, onShowAllChange, onOpenChat, onCreateNew }: Props = $props();
</script>

<div class="space-y-4">
  <div class="flex items-center gap-4">
    <Checkbox
      checked={showAll}
      onchange={(event) => onShowAllChange((event.currentTarget as HTMLInputElement).checked)}
      class="text-gray-400 font-medium uppercase text-[10px] tracking-widest"
    >
      Show Archived
    </Checkbox>
  </div>

  <div class="flex justify-end gap-2 -mt-14 mb-8 relative z-50 px-4">
    <Button color="alternative" class="bg-black border-gray-800 text-[#86BC25] hover:bg-white/5 rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={onOpenChat}>
      <MessagesOutline size="xs" class="mr-1.5" />
      Draft with AI
    </Button>

    <Button color="alternative" class="bg-[#86BC25] text-black rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={onCreateNew}>
      <PlusOutline size="xs" class="mr-1.5" />
      New Doc
    </Button>
  </div>
</div>
