<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { postJSON, requestJSON, deleteJSON, fetchJSON } from '$lib/api';
	import { isProviderTypeEnabled } from '$lib/feature-flags';
	import { slugifySegment } from '$lib/utils';
  import { Drawer, Button, Label } from 'flowbite-svelte';
  import { ArchiveOutline, TrashBinOutline, CheckOutline, CloseOutline } from 'flowbite-svelte-icons';
  import { sineIn } from 'svelte/easing';
  import ProjectBasicsSection from '$lib/components/ProjectBasicsSection.svelte';
  import ProjectAuthSection from '$lib/components/ProjectAuthSection.svelte';
  import ProjectRuntimeSection from '$lib/components/ProjectRuntimeSection.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
		onSaved?: () => void;
		projectToEdit?: any;
	}

	let { open = $bindable(), onClose, onSaved = () => {}, projectToEdit = null }: Props = $props();

	let isEditing = $derived(!!projectToEdit);
	
	// Form fields
	let id = $state('');
	let name = $state('');
	let repoUrl = $state('');
	let githubUser = $state('');
	let githubCredential = $state('');
	let providerProfileID = $state('codex-default');
	let providerProfiles = $state<any[]>([]);
	let runtimeImage = $state('');
	let skillsImage = $state('');
	let credentialTestBusy = $state(false);
	let credentialTestResult = $state<{ valid: boolean; message: string } | null>(null);
	let busy = $state(false);

  // Use bind:hidden for drawer control
  let isHidden = $state(true);
  $effect(() => { isHidden = !open; });
  $effect(() => { if (open && isHidden) onClose(); });

  let transitionParams = {
    x: 450,
    duration: 300,
    easing: sineIn
  };

	$effect(() => {
		if (open) {
			void loadProviderProfiles();
			if (projectToEdit) {
				id = projectToEdit.id || '';
				name = projectToEdit.name || '';
				repoUrl = projectToEdit.repo_url || '';
				providerProfileID = projectToEdit.provider_profile_id || 'codex-default';
				githubUser = projectToEdit.github_user || '';
				runtimeImage = projectToEdit.runtime_image || '';
				skillsImage = projectToEdit.skills_image || '';
				githubCredential = '';
			} else {
				id = '';
				name = '';
				repoUrl = '';
				providerProfileID = 'codex-default';
				githubUser = '';
				githubCredential = '';
				runtimeImage = '';
				skillsImage = '';
			}
		}
	});

	async function loadProviderProfiles() {
		try {
			const profiles = await fetchJSON('/v1/providers');
			providerProfiles = Array.isArray(profiles)
				? profiles.filter((profile) => isProviderTypeEnabled(String(profile?.provider_type || profile?.id || '')))
				: [];
		} catch {
			providerProfiles = [];
		}
	}

	async function saveProject() {
		if (!name || !repoUrl) {
			pushToast("Name and Repo URL are required", "err");
			return;
		}
		busy = true;
		const projectId = isEditing ? id : slugifySegment(name);
		const projectPayload = {
			id: projectId,
			name,
			repo_url: repoUrl,
			provider_profile_id: providerProfileID || 'codex-default',
			github_user: githubUser,
			runtime_image: runtimeImage,
			skills_image: skillsImage,
			runtime_pull_policy: 'IfNotPresent',
			skills_pull_policy: 'IfNotPresent'
		};
		try {
			credentialTestResult = null;
			if (isEditing) {
				await requestJSON(`/v1/projects/${projectId}`, "PUT", projectPayload);
			} else {
				await postJSON("/v1/projects", projectPayload);
			}
			if (githubCredential) {
				await postJSON("/v1/projects/credentials/github", {
					actor: "operator", project_id: projectId, github_user: githubUser, credential: githubCredential
				});
			}
			pushToast(`Project ${isEditing ? 'updated' : 'created'} successfully`, "ok");
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
			onSaved();
			onClose();
		} catch (err: any) {
			pushToast(err.message || "Failed to save project", "err");
		} finally {
			busy = false;
		}
	}

	async function testProjectCredential() {
		const projectId = isEditing ? id : slugifySegment(name);
		if (projectId.trim() === '') {
			pushToast('Save project details first to test credentials', 'err');
			return;
		}
		credentialTestBusy = true;
		credentialTestResult = null;
		try {
			const response = await postJSON('/v1/projects/credentials/github/test', {
				actor: 'operator',
				project_id: projectId
			});
			credentialTestResult = {
				valid: !!response?.valid,
				message: String(response?.message || (response?.valid ? 'Repository access verified' : 'Repository access check failed'))
			};
			if (response?.valid) {
				pushToast('Credential test passed', 'ok');
			} else {
				pushToast(credentialTestResult.message, 'err');
			}
		} catch (err: any) {
			credentialTestResult = {
				valid: false,
				message: err?.message || 'Failed to test project credential'
			};
			pushToast(credentialTestResult.message, 'err');
		} finally {
			credentialTestBusy = false;
		}
	}

	async function deleteProject() {
		if (!confirm(`Are you sure you want to delete project ${name}?`)) return;
		busy = true;
		try {
			await deleteJSON(`/v1/projects/${id}`);
			pushToast("Project deleted", "ok");
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
			onSaved();
			onClose();
		} catch (err: any) {
			pushToast(err.message || "Failed to delete project", "err");
		} finally {
			busy = false;
		}
	}
</script>

<Drawer 
  placement="right" 
  bind:hidden={isHidden} 
  outsideclose={true}
  id="project-editor-drawer" 
  width="default" 
  class="fixed top-0 right-0 bg-black border-l border-gray-800 z-50 overflow-y-auto h-full m-0 shadow-2xl p-0 w-[450px]"
>
  <div class="flex flex-col h-full relative">
    <!-- Header -->
    <div class="px-8 py-10 bg-slate-900/20 border-b border-gray-900 flex items-center justify-between">
      <div class="flex items-center gap-4">
        <div class="w-12 h-12 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
          <ArchiveOutline size="lg" />
        </div>
        <div>
          <h2 class="text-2xl font-bold text-white uppercase tracking-tighter">{isEditing ? 'Edit Project' : 'New Project'}</h2>
          <p class="text-[10px] font-bold text-gray-500 uppercase tracking-[0.2em] mt-1">Configuration & Runtime</p>
        </div>
      </div>
      <button 
        class="text-white hover:text-[#86BC25] transition-colors p-2"
        onclick={onClose}
        aria-label="Close Drawer"
      >
        <CloseOutline size="md" />
      </button>
    </div>

    <!-- Body -->
    <form class="flex-1 p-8 space-y-8" onsubmit={(e) => { e.preventDefault(); saveProject(); }}>
      <div class="space-y-6">
        <ProjectBasicsSection
          {name}
          {repoUrl}
          {busy}
          onNameChange={(value) => name = value}
          onRepoUrlChange={(value) => repoUrl = value}
        />

				<div>
					<Label class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Provider Profile</Label>
					<select
						class="w-full bg-black border border-gray-800 text-white text-sm rounded-none px-3 py-2"
						value={providerProfileID}
						oninput={(event) => providerProfileID = (event.currentTarget as HTMLSelectElement).value}
						disabled={busy}
					>
						{#if providerProfiles.length === 0}
							<option value="codex-default">codex-default</option>
						{:else}
							{#each providerProfiles as profile}
								<option value={profile.id}>{profile.name || profile.id}</option>
							{/each}
						{/if}
					</select>
				</div>

        <div class="my-8 border-t border-gray-900"></div>

        <ProjectAuthSection
          {githubUser}
          {githubCredential}
          {busy}
          onGitHubUserChange={(value) => githubUser = value}
          onGitHubCredentialChange={(value) => githubCredential = value}
        />

				<div class="space-y-2">
					<Button
						color="alternative"
						size="xs"
						data-testid="project-credential-test"
						class="rounded-none border-gray-700 bg-slate-900 text-gray-300"
						onclick={testProjectCredential}
						disabled={busy || credentialTestBusy || !isEditing}
					>
						Test Repository Access
					</Button>
					<p class="text-[10px] uppercase tracking-widest text-gray-600">
						{#if isEditing}
							Uses the stored project PAT to verify repository access.
						{:else}
							Save project first to enable credential test.
						{/if}
					</p>
					{#if credentialTestResult}
						<p class={`text-[11px] ${credentialTestResult.valid ? 'text-[#86BC25]' : 'text-red-400'}`}>
							{credentialTestResult.message}
						</p>
					{/if}
				</div>

        <div class="my-8 border-t border-gray-900"></div>

        <ProjectRuntimeSection
          {runtimeImage}
          {skillsImage}
          {busy}
          onRuntimeImageChange={(value) => runtimeImage = value}
          onSkillsImageChange={(value) => skillsImage = value}
        />
      </div>

      <!-- Footer Actions -->
      <div class="pt-10 pb-20 border-t border-gray-900 flex justify-between gap-4 mt-auto">
        {#if isEditing}
          <Button color="red" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest px-6" onclick={deleteProject} disabled={busy}>
            <TrashBinOutline size="xs" class="mr-2" />
            Delete
          </Button>
        {/if}
        <div class="flex gap-3 ml-auto">
          <Button color="alternative" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest border-gray-700 bg-slate-800 text-gray-300 hover:bg-slate-700 px-6" onclick={onClose} disabled={busy}>Cancel</Button>
          <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest rounded-none px-8 py-2 hover:bg-[#a1e02c]" type="submit" disabled={busy}>
            <CheckOutline size="xs" class="mr-2" />
            {isEditing ? 'Update Project' : 'Create Project'}
          </Button>
        </div>
      </div>
    </form>
  </div>
</Drawer>

<style>
  :global(#project-editor-drawer) {
    background-color: #000000 !important;
    left: auto !important;
    right: 0 !important;
  }
  /* Force hide Flowbite's default absolute-positioned close button */
  :global(#project-editor-drawer button[aria-label="Close"]) {
    display: none !important;
  }
  /* Preserve custom close button visibility */
  :global(#project-editor-drawer .relative > button[aria-label="Close Drawer"]) {
    display: flex !important;
  }
</style>
