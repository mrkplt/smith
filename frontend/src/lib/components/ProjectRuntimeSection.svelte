<script lang="ts">
  import { Input, Label } from 'flowbite-svelte';

  interface Props {
    runtimeImage: string;
    skillsImage: string;
    busy: boolean;
    onRuntimeImageChange: (value: string) => void;
    onSkillsImageChange: (value: string) => void;
  }

  let { runtimeImage, skillsImage, busy, onRuntimeImageChange, onSkillsImageChange }: Props = $props();
</script>

<div class="space-y-6">
  <h4 class="text-[10px] font-bold uppercase tracking-[0.3em] text-[#86BC25]">Runtime & Skills</h4>
  <div class="grid grid-cols-1 gap-4">
    <div>
      <Label for="r-image" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Replica Image</Label>
      <Input
        type="text"
        id="r-image"
        placeholder="smith-replica:latest"
        value={runtimeImage}
        oninput={(event) => onRuntimeImageChange((event.currentTarget as HTMLInputElement).value)}
        disabled={busy}
        class="bg-black border-gray-800 text-white font-mono text-sm rounded-none focus:border-[#86BC25]"
      />
    </div>
    <div>
      <Label for="s-image" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Skills Image</Label>
      <Input
        type="text"
        id="s-image"
        placeholder="smith-skills:latest"
        value={skillsImage}
        oninput={(event) => onSkillsImageChange((event.currentTarget as HTMLInputElement).value)}
        disabled={busy}
        class="bg-black border-gray-800 text-white font-mono text-sm rounded-none focus:border-[#86BC25]"
      />
    </div>
  </div>
</div>
