<script lang="ts">
  import { Button, Input } from 'flowbite-svelte';
  import { PaperPlaneOutline } from 'flowbite-svelte-icons';

  interface Props {
    chatInput: string;
    disabled: boolean;
    onChatInputChange: (value: string) => void;
    onSend: () => void;
  }

  let { chatInput, disabled, onChatInputChange, onSend }: Props = $props();
</script>

<div class="flex gap-2">
  <Input
    type="text"
    placeholder="Describe your document needs..."
    value={chatInput}
    oninput={(event) => onChatInputChange((event.currentTarget as HTMLInputElement).value)}
    disabled={disabled}
    onkeydown={(event) => event.key === 'Enter' && onSend()}
    class="bg-slate-900 border-gray-800 text-white rounded-none"
  />
  <Button color="alternative" class="bg-[#86BC25] text-black px-4 rounded-none" onclick={onSend} disabled={disabled || !chatInput}>
    <PaperPlaneOutline size="sm" />
  </Button>
</div>
