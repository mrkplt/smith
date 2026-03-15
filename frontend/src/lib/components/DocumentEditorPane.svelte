<script lang="ts">
  import { Textarea, Toolbar, ToolbarButton, ToolbarGroup } from 'flowbite-svelte';

  interface Props {
    editContent: string;
    onEditContent: (value: string) => void;
  }

  let { editContent, onEditContent }: Props = $props();

  function appendToContent(fragment: string) {
    onEditContent(editContent + fragment);
  }
</script>

<div class="flex-1 flex flex-col bg-black">
  <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest">Editor</div>
  <Textarea
    value={editContent}
    oninput={(event) => onEditContent((event.currentTarget as HTMLTextAreaElement).value)}
    rows={20}
    placeholder="Start writing in Markdown..."
    class="bg-black border-none text-gray-300 font-mono text-lg focus:ring-0 resize-none h-full w-full rounded-none px-8 py-6"
  >
    <Toolbar slot="header" embedded class="bg-black border-b border-gray-900 rounded-none px-8">
      <ToolbarGroup>
        <ToolbarButton name="Bold" class="text-gray-400 hover:text-white" onclick={() => appendToContent('**bold**')}><span class="font-bold text-xs">B</span></ToolbarButton>
        <ToolbarButton name="Italic" class="text-gray-400 hover:text-white" onclick={() => appendToContent('_italic_')}><span class="italic text-xs">I</span></ToolbarButton>
        <ToolbarButton name="List" class="text-gray-400 hover:text-white" onclick={() => appendToContent('\n- ')}><span class="text-xs">L</span></ToolbarButton>
      </ToolbarGroup>
    </Toolbar>
  </Textarea>
</div>
