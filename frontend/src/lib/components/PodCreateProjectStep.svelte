<script lang="ts">
  import { Label, Select } from 'flowbite-svelte';

  interface Props {
    method: string;
    projectID: string;
    issueNumber: string;
    projects: any[];
    issues: any[];
    onProjectChange: (projectID: string) => void;
    onIssueChange: (issueNumber: string) => void;
  }

  let { method, projectID, issueNumber, projects, issues, onProjectChange, onIssueChange }: Props = $props();
</script>

<div class="space-y-4">
  <div>
    <Label for="project" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Target Project</Label>
    <Select
      id="project"
      value={projectID}
      onchange={(event) => onProjectChange((event.currentTarget as HTMLSelectElement).value)}
      class="bg-slate-900 border-gray-800 text-white rounded-none"
    >
      <option value="">Select a project</option>
      {#each projects as p}
        <option value={p.id}>{p.name}</option>
      {/each}
    </Select>
  </div>
  {#if method === 'issue'}
    <div>
      <Label for="issue" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">GitHub Issue</Label>
      <Select
        id="issue"
        value={issueNumber}
        onchange={(event) => onIssueChange((event.currentTarget as HTMLSelectElement).value)}
        disabled={issues.length === 0}
        class="bg-slate-900 border-gray-800 text-white rounded-none"
      >
        <option value="">{issues.length === 0 ? 'No issues found' : 'Select an issue'}</option>
        {#each issues as issue}
          <option value={issue.number}>#{issue.number} {issue.title}</option>
        {/each}
      </Select>
    </div>
  {/if}
</div>
