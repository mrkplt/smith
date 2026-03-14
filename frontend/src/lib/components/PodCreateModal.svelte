<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { fetchJSON, postJSON } from '$lib/api';
	import { onDestroy } from 'svelte';
	import { slugifySegment } from '$lib/utils';
  import { Modal, Button, Badge } from 'flowbite-svelte';
  import { ArrowLeftOutline, ArrowRightOutline, RocketOutline } from 'flowbite-svelte-icons';
  import PodCreateMethodStep from '$lib/components/PodCreateMethodStep.svelte';
  import PodCreateProjectStep from '$lib/components/PodCreateProjectStep.svelte';
  import PodCreateDetailsStep from '$lib/components/PodCreateDetailsStep.svelte';
  import PodCreatePRDChatStep from '$lib/components/PodCreatePRDChatStep.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open = $bindable(), onClose }: Props = $props();

	let step = $state(1);
	let method = $state('issue');
	let projectID = $state('');
	let issueNumber = $state('');
	let loopName = $state('');
	let branch = $state('');
	let sourceBranch = $state('main');
	let provider = $state('');
	let prompt = $state('');
	let busy = $state(false);
	
	let chatMessages = $state<{ type: string, text?: string, error?: string, final_prd_path?: string }[]>([]);
	let chatSocket: WebSocket | null = $state(null);
	let chatInput = $state('');
	let finalPRD = $state<string | null>(null);

	const isInteractive = $derived(method === 'issue' || method === 'generate_prd');
	const maxStep = $derived(isInteractive ? 4 : 3);

	const projects = $derived($appState.projects);
	const issues = $derived(projectID ? ($appState.podProjectIssues[projectID] || []) : []);

	async function loadIssues() {
		if (!projectID) return;
		busy = true;
		try {
			const raw = await fetchJSON("/v1/projects/" + projectID + "/issues");
			appState.update(s => ({
				...s,
				podProjectIssues: { ...s.podProjectIssues, [projectID]: Array.isArray(raw) ? raw : [] }
			}));
		} catch (err) {
			console.error("Failed to load issues", err);
		} finally {
			busy = false;
		}
	}

	function nextStep() {
		if (step === 2 && !projectID) {
			pushToast("Select a project first.", "err");
			return;
		}
		if (step === 3 && isInteractive) {
			startPRDChat();
		}
		step = Math.min(maxStep, step + 1);
	}

	function startPRDChat() {
		if (chatSocket) chatSocket.close();
		
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		const url = `${protocol}//${host}/api/v1/chat/prd`;

		chatSocket = new WebSocket(url);
		chatMessages = [];
		finalPRD = null;

		chatSocket.onopen = () => {
			let initialMsg = prompt;
			if (!initialMsg && issueNumber) {
				const issue = issues.find(i => String(i.number) === issueNumber);
				if (issue) {
					initialMsg = `Build a PRD from GitHub issue #${issue.number}: ${issue.title}\n\n${issue.body || ""}`;
				}
			}
			if (!initialMsg) initialMsg = "Let's build a new PRD.";
			chatSocket?.send(JSON.stringify({ type: 'user', text: initialMsg }));
		};

		chatSocket.onmessage = (event) => {
			try {
				const msg = JSON.parse(event.data);
				if (msg.type === 'system' && msg.final_prd_path) {
					finalPRD = msg.text;
				}
				chatMessages = [...chatMessages, msg];
			} catch (err) {
				console.error("Failed to parse chat message", err);
			}
		};

		chatSocket.onclose = () => {
			chatSocket = null;
		};
	}

	function sendChatMessage() {
		if (!chatInput || !chatSocket) return;
		chatSocket.send(JSON.stringify({ type: 'user', text: chatInput }));
		chatMessages = [...chatMessages, { type: 'user', text: chatInput }];
		chatInput = '';
	}

	function prevStep() {
		step = Math.max(1, step - 1);
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
	}

	async function submit() {
		busy = true;
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
		try {
			const payload = {
				loop_id: slugifySegment(projectID + "-" + (loopName || "loop")),
				title: "Loop " + (loopName || "request"),
				provider_id: provider,
				source_type: method === 'issue' ? 'github_issue' : 'prompt',
				source_ref: method === 'issue' ? `${projectID}#${issueNumber}` : 'prompt',
				metadata: {
					project_id: projectID,
					workspace_branch: branch,
					workspace_source_branch: sourceBranch,
					workspace_prd_json: finalPRD || (method === 'load_prd' ? prompt : ""),
				}
			};
			await postJSON("/v1/loops", payload);
			pushToast("Loop created successfully.", "ok");
			onClose();
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}

  $effect(() => {
    if (!open) {
      step = 1;
      if (chatSocket) {
				chatSocket.close();
				chatSocket = null;
			}
    }
  });

	onDestroy(() => {
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
	});
</script>

<Modal bind:open title="Start New Loop" size="md" autoclose={false} class="bg-black border border-gray-800 rounded-none">
  <div class="space-y-6">
    <div class="flex justify-between items-center mb-4">
      <div class="flex gap-2">
        {#each Array.from({ length: maxStep }) as _, i}
          <div class="w-8 h-1 rounded-full {step > i ? 'bg-[#86BC25]' : 'bg-slate-800'}"></div>
        {/each}
      </div>
      <Badge color="gray" class="text-[10px] uppercase font-bold tracking-wider rounded-none bg-slate-800 text-gray-400">Step {step} of {maxStep}</Badge>
    </div>

    {#if step === 1}
      <PodCreateMethodStep
        {method}
        onSelectMethod={(value) => method = value}
      />
    {:else if step === 2}
      <PodCreateProjectStep
        {method}
        {projectID}
        {issueNumber}
        {projects}
        {issues}
        onProjectChange={async (value) => {
          projectID = value;
          issueNumber = '';
          await loadIssues();
        }}
        onIssueChange={(value) => issueNumber = value}
      />
    {:else if step === 3}
      <PodCreateDetailsStep
        {method}
        {loopName}
        {branch}
        {prompt}
        onLoopNameChange={(value) => loopName = value}
        onBranchChange={(value) => branch = value}
        onPromptChange={(value) => prompt = value}
      />
    {:else if step === 4}
      <PodCreatePRDChatStep
        {chatMessages}
        {finalPRD}
        {chatSocket}
        {chatInput}
        onChatInputChange={(value) => chatInput = value}
        onSendChatMessage={sendChatMessage}
      />
    {/if}
  </div>

  <svelte:fragment slot="footer">
    <Button color="alternative" class="rounded-none border-gray-700" onclick={onClose}>Cancel</Button>
    <div class="flex gap-2 ml-auto">
      <Button color="alternative" class="rounded-none border-gray-700" onclick={prevStep} disabled={step === 1}>
        <ArrowLeftOutline size="sm" class="mr-2" /> Back
      </Button>
      {#if step < maxStep}
        <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-xs px-6 py-2 rounded-none transition-all" onclick={nextStep}>
          Next <ArrowRightOutline size="sm" class="ml-2" />
        </Button>
      {:else}
        <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-xs px-6 py-2 rounded-none transition-all" onclick={submit} disabled={busy || (isInteractive && !finalPRD)}>
          <RocketOutline size="sm" class="mr-2" /> Create Loop
        </Button>
      {/if}
    </div>
  </svelte:fragment>
</Modal>
