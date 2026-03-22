<script lang="ts">
	import { page } from '$app/state';
	import { appState, pushToast } from '$lib/stores';
	import { onDestroy, onMount } from 'svelte';
	import { attachLoopTerminal, cancelLoop, createLoopIntervention, detachLoopTerminal, getJSON, pauseLoop, postJSON, resumeLoop, sendLoopCommand } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import Journal from '$lib/components/Journal.svelte';
	import { goto } from '$app/navigation';
  import { Badge, Button, Input } from 'flowbite-svelte';
  import { ArrowLeftOutline, TrashBinOutline, TerminalOutline } from 'flowbite-svelte-icons';

	const id = $derived(page.params.id);
	const liveLoop = $derived($appState.loops.find((loop: any) => loop.loopID === id) || null);

	let loopState = $state('unknown');
	let loopReason = $state('');
	
	let command = $state('');
	let intervention = $state('');
	let busy = $state(false);
	let terminalAttached = $state(false);

	const terminalActor = 'operator';
	const terminalName = 'console-pods';

	const isRunning = $derived(loopState === 'running');
	const isUnresolved = $derived(loopState === 'unresolved');
	const isTerminal = $derived(loopState === 'synced' || loopState === 'flatline' || loopState === 'cancelled');
	const lifecycle = $derived.by(() => {
		if (loopState === 'running') {
			return { label: 'active', color: 'green' as const };
		}
		if (loopState === 'unresolved') {
			return { label: 'paused', color: 'yellow' as const };
		}
		if (loopState === 'synced' || loopState === 'flatline' || loopState === 'cancelled') {
			return { label: 'terminal', color: 'gray' as const };
		}
		return { label: 'unknown', color: 'gray' as const };
	});
	const stateColor = $derived.by(() => {
		switch (loopState) {
			case 'running':
				return 'green' as const;
			case 'unresolved':
				return 'yellow' as const;
			case 'synced':
				return 'blue' as const;
			case 'flatline':
				return 'red' as const;
			case 'cancelled':
				return 'gray' as const;
			default:
				return 'gray' as const;
		}
	});
	const canPause = $derived(isRunning);
	const canResume = $derived(isUnresolved);
	const canCancel = $derived(isRunning || isUnresolved);
	const canTerminate = $derived(isRunning);

	onMount(async () => {
		await refreshLoop();
		await ensureTerminalAttach();
	});

	onDestroy(() => {
		void detachTerminalSession();
	});

	$effect(() => {
		if (liveLoop) {
			loopState = String((liveLoop as any).status || 'unknown').toLowerCase();
			loopReason = String((liveLoop as any).reason || '');
		}
	});

	$effect(() => {
		if (!isAttachableLoopState(loopState) && terminalAttached) {
			void detachTerminalSession();
		}
	});

	function isAttachableLoopState(state: string) {
		const value = String(state || '').toLowerCase();
		return value === 'running' || value === 'unresolved';
	}

	async function ensureTerminalAttach() {
		if (!id || terminalAttached || !isAttachableLoopState(loopState)) return;
		try {
			await attachLoopTerminal(id, { actor: terminalActor, terminal: terminalName });
			terminalAttached = true;
		} catch (err: any) {
			const message = String(err?.message || 'terminal attach failed');
			if (!message.toLowerCase().includes('loop is not active')) {
				pushToast(message, 'err');
			}
		}
	}

	async function detachTerminalSession() {
		if (!id || !terminalAttached) return;
		try {
			await detachLoopTerminal(id, { actor: terminalActor });
		} catch (err: any) {
			const message = String(err?.message || '');
			if (!message.toLowerCase().includes('actor is not attached')) {
				console.warn('terminal detach failed', err);
			}
		} finally {
			terminalAttached = false;
		}
	}

	async function refreshLoop() {
		if (!id) return;
		try {
			const response = await getJSON(`/v1/loops/${encodeURIComponent(id)}`);
			const state = response?.state || {};
			const status = String(state.state || state.State || 'unknown').toLowerCase();
			const reason = String(state.reason || state.Reason || '');
			const displayTitle = String(state.display_title || state.DisplayTitle || id || '');
			const attempt = Number(state.attempt || state.Attempt || 0);
			const currentCount = Number(state.current_count || state.CurrentCount || attempt || 0);
			const targetCount = Number(state.target_count || state.TargetCount || currentCount || 1);
			const revision = Number(state.observed_revision || state.ObservedRevision || 0);
			loopState = status;
			loopReason = reason;
			appState.update((s) => {
				const loops = [...s.loops];
				const idx = loops.findIndex((loop: any) => loop.loopID === id);
				const normalized = {
					loopID: id,
					displayTitle,
					currentCount,
					targetCount,
					project: loops[idx]?.project || 'default',
					status,
					attempt,
					reason,
					revision,
				};
				if (idx >= 0) {
					loops[idx] = normalized as never;
				} else {
					loops.push(normalized as never);
				}
				return { ...s, loops };
			});
		} catch {
			// Best effort refresh; stream remains primary source.
		}
	}

	async function runCommand() {
		if (!command || !id) return;
		busy = true;
		try {
			if (!terminalAttached) {
				await ensureTerminalAttach();
			}
			if (!terminalAttached) {
				pushToast('Terminal is not attached', 'err');
				return;
			}
			await sendLoopCommand(id, { actor: terminalActor, command });
			command = '';
			pushToast("Command sent", "ok");
		} catch (err: any) {
			const message = String(err?.message || 'command failed');
			if (message.toLowerCase().includes('actor is not attached')) {
				terminalAttached = false;
				await ensureTerminalAttach();
			}
			pushToast(message, "err");
		} finally {
			busy = false;
		}
	}

	async function terminate() {
		if (!canTerminate) {
			pushToast('Terminate is only available for running pods', 'muted');
			return;
		}
		if (!confirm("Force terminate this loop? (State will be set to flatline)")) return;
		busy = true;
		try {
			await postJSON('/v1/control/override', {
				loop_id: id,
				target_state: "flatline",
				reason: "terminated via console",
				actor: "operator",
			});
			pushToast("Termination requested", "ok");
			await refreshLoop();
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}

	async function cancel() {
		if (!canCancel) {
			pushToast('Cancel is only available for active pods', 'muted');
			return;
		}
		if (!confirm("Cancel this loop?")) return;
		busy = true;
		try {
			await cancelLoop(id, { actor: 'operator', reason: 'cancelled via console' });
			pushToast("Cancellation requested", "ok");
			await refreshLoop();
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}

	async function pause() {
		if (!canPause) {
			pushToast('Pause is only available for running pods', 'muted');
			return;
		}
		busy = true;
		try {
			await pauseLoop(id, { actor: 'operator', reason: 'paused via console' });
			pushToast('Loop paused', 'ok');
			await refreshLoop();
		} catch (err: any) {
			pushToast(err.message, 'err');
		} finally {
			busy = false;
		}
	}

	async function resume() {
		if (!canResume) {
			pushToast('Resume is only available for unresolved pods', 'muted');
			return;
		}
		busy = true;
		try {
			await resumeLoop(id, { actor: 'operator', reason: 'resumed via console' });
			pushToast('Loop resumed', 'ok');
			await refreshLoop();
		} catch (err: any) {
			pushToast(err.message, 'err');
		} finally {
			busy = false;
		}
	}

	async function sendIntervention() {
		if (!intervention.trim() || !id) return;
		busy = true;
		try {
			await createLoopIntervention(id, {
				actor: 'operator',
				instruction: intervention.trim(),
				event_id: `console-${Date.now()}`,
				type: 'operator_intervention',
			});
			intervention = '';
			pushToast('Intervention recorded', 'ok');
			await refreshLoop();
		} catch (err: any) {
			pushToast(err.message, 'err');
		} finally {
			busy = false;
		}
	}

</script>

<TopBar title={`Pod: ${id}`} />

<div class="pod-detail-shell px-4 mb-6 space-y-3">
	<div class="pod-surface-row">
		<div class="pod-state-strip">
			<span class="pod-chip-label">State</span>
			<Badge color={stateColor} rounded class="uppercase text-[10px] px-2 py-0.5 font-bold">{loopState}</Badge>
			<span class="pod-chip-label">Lifecycle</span>
			<Badge color={lifecycle.color} rounded class="uppercase text-[10px] px-2 py-0.5 font-bold">{lifecycle.label}</Badge>
			<span class="pod-chip-label">Terminal</span>
			<span class={terminalAttached ? 'text-[#86BC25]' : 'text-amber-400'}>{terminalAttached ? 'attached' : 'detached'}</span>
			{#if loopReason}
				<span class="pod-reason">{loopReason}</span>
			{/if}
		</div>
    <Button color="alternative" class="pod-btn secondary" onclick={refreshLoop} disabled={busy}>
      Refresh
    </Button>
  </div>

	<div class="pod-surface-row">
		<Button color="alternative" class="pod-btn secondary" onclick={() => goto('/pods')}>
			<ArrowLeftOutline size="xs" class="mr-1.5" />
			Back
		</Button>
		<div class="pod-action-group">
			{#if canPause}
				<Button color="alternative" class="pod-btn secondary" onclick={pause} disabled={busy}>
					Pause
				</Button>
			{/if}
			{#if canResume}
				<Button color="alternative" class="pod-btn secondary" onclick={resume} disabled={busy}>
					Resume
				</Button>
			{/if}
			{#if canCancel}
				<Button color="alternative" class="pod-btn secondary" onclick={cancel} disabled={busy}>
					Cancel
				</Button>
			{/if}
			{#if canTerminate}
				<Button color="red" class="pod-btn danger" onclick={terminate} disabled={busy}>
					<TrashBinOutline size="xs" class="mr-1.5" />
					Terminate
				</Button>
			{/if}
		</div>
	</div>
</div>

<div class="px-4">
  <Journal loopID={id || ""}>
    <div class="pod-command-row">
      <div class="flex items-center gap-2 pl-2 text-[#86BC25]">
        <TerminalOutline size="xs" />
        <span class="font-mono font-bold">$</span>
      </div>
      <Input
        type="text"
        placeholder="Run command (e.g. ls -la)"
        bind:value={command}
        onkeydown={(e) => e.key === 'Enter' && runCommand()}
        disabled={busy}
        size="sm"
        class="pod-command-input"
      />
      <Button size="xs" color="alternative" class="pod-btn execute" onclick={runCommand} disabled={busy || !command}>
        {busy ? '...' : 'Execute'}
      </Button>
    </div>
    <div class="pod-command-row pod-command-row-tail">
      <div class="flex items-center gap-2 pl-2 text-blue-400">
        <span class="font-mono font-bold">!</span>
      </div>
      <Input
        type="text"
        placeholder="Record operator intervention instruction"
        bind:value={intervention}
        onkeydown={(e) => e.key === 'Enter' && sendIntervention()}
        disabled={busy}
        size="sm"
        class="pod-command-input"
      />
      <Button size="xs" color="alternative" class="pod-btn info" onclick={sendIntervention} disabled={busy || !intervention.trim()}>
        Record
      </Button>
    </div>
  </Journal>
</div>

<style>
  .pod-surface-row {
    background: var(--surface-2);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    box-shadow: var(--elevation-1), var(--inner-highlight);
    padding: 0.55rem 0.7rem;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.7rem;
  }

  .pod-state-strip {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .pod-chip-label {
    color: #64748b;
  }

  .pod-reason {
    color: #64748b;
    text-transform: none;
    letter-spacing: 0;
    font-family: var(--mono);
    font-size: 0.65rem;
    overflow-wrap: anywhere;
  }

  .pod-action-group {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: 0.45rem;
  }

  :global(.pod-btn) {
    border-radius: 8px !important;
    border: 1px solid var(--border-subtle) !important;
    font-weight: 700 !important;
    text-transform: uppercase;
    letter-spacing: 0.12em !important;
    font-size: 0.6rem !important;
    height: 1.75rem;
  }

  :global(.pod-btn.secondary) {
    background: var(--surface-1) !important;
    color: #475569 !important;
  }

  :global(.pod-btn.execute) {
    background: #86bc25 !important;
    color: #0f172a !important;
    border-color: rgba(134, 188, 37, 0.6) !important;
  }

  :global(.pod-btn.info) {
    background: #3b82f6 !important;
    color: #0f172a !important;
    border-color: rgba(59, 130, 246, 0.6) !important;
  }

  :global(.pod-btn.danger) {
    border-color: rgba(239, 68, 68, 0.55) !important;
    background: rgba(254, 226, 226, 0.72) !important;
    color: #b91c1c !important;
  }

  .pod-command-row {
    background: var(--surface-2);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    padding: 0.5rem;
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .pod-command-row-tail {
    border-top-left-radius: 0;
    border-top-right-radius: 0;
    border-top: none;
    margin-top: -1px;
  }

  :global(.pod-command-input) {
    background: transparent !important;
    border: none !important;
    color: #334155 !important;
    font-family: var(--mono) !important;
  }

  :global(.pod-command-row input) {
    background: transparent !important;
  }

  :global(.dark .pod-chip-label),
  :global(.dark .pod-reason) {
    color: #94a3b8;
  }

  :global(.dark .pod-btn.secondary) {
    color: #cbd5e1 !important;
  }

  :global(.dark .pod-btn.danger) {
    color: #fca5a5 !important;
    background: rgba(127, 29, 29, 0.38) !important;
  }

  :global(.dark .pod-command-input) {
    color: #e2e8f0 !important;
  }
</style>
