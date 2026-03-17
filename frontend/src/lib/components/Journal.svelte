<script lang="ts">
	import { onDestroy } from 'svelte';
	import { appState } from '$lib/stores';
	import type { Snippet } from 'svelte';
	import { appendJournalEntry, openJournalStream, renderJournalText } from '$lib/journal/stream';

	interface Props {
		loopID: string;
		children?: Snippet;
	}

	let { loopID, children }: Props = $props();

	let terminalEl: HTMLPreElement | null = $state(null);

	function percentile(samples: number[], p: number): number | null {
		if (samples.length === 0) return null;
		const sorted = [...samples].sort((a, b) => a - b);
		const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil((p / 100) * sorted.length) - 1));
		return sorted[idx];
	}

	const latencyP95Ms = $derived(percentile($appState.latencySamplesMs, 95));

	function renderJournal() {
		if (!terminalEl) return;
		terminalEl.textContent = renderJournalText($appState.journalEntries);
		terminalEl.scrollTop = terminalEl.scrollHeight;
	}

	let source: EventSource | null = null;
	let reconnectTimer: any = null;

	function connect() {
		if (reconnectTimer) clearTimeout(reconnectTimer);
		if (source) source.close();
		if (!loopID) return;
		source = openJournalStream(
			loopID,
			entry => {
				appendJournalEntry(entry);
				renderJournal();
			},
			() => {
				source = null;
				reconnectTimer = setTimeout(connect, 3000);
			}
		);
	}

  $effect(() => {
		appState.update(s => ({ ...s, journalEntries: [], journalLastSeq: 0, latencySamplesMs: [] }));
		connect();
  });

	onDestroy(() => {
		if (reconnectTimer) clearTimeout(reconnectTimer);
		if (source) source.close();
	});
</script>

<div class="waypoint-terminal-window">
	<div class="terminal-header">
		<div class="terminal-traffic-lights">
			<span class="light close"></span>
			<span class="light minimize"></span>
			<span class="light maximize"></span>
		</div>
		<div class="terminal-title">Live Journal: {loopID}</div>
		<div class={`latency-chip ${latencyP95Ms !== null && latencyP95Ms >= 100 ? 'warn' : 'ok'}`}>
			p95 {latencyP95Ms === null ? '--' : `${latencyP95Ms}ms`}
		</div>
	</div>
	<div class="terminal-scroll-area">
		<pre bind:this={terminalEl} class="terminal-body">[journal] attaching to stream...</pre>
		{#if children}
			<div class="terminal-input-area">
				{@render children()}
			</div>
		{/if}
	</div>
</div>

<style>
	.waypoint-terminal-window {
		background: #0f111a;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 8px;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
		margin-bottom: 12px;
	}

	.terminal-header {
		background: #1b1e28;
		padding: 10px 16px;
		display: flex;
		align-items: center;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		position: relative;
	}

	.terminal-traffic-lights {
		display: flex;
		gap: 6px;
	}

	.light {
		width: 12px;
		height: 12px;
		border-radius: 50%;
	}
	.light.close { background: #ff5f56; }
	.light.minimize { background: #ffbd2e; }
	.light.maximize { background: #27c93f; }

	.terminal-title {
		position: absolute;
		left: 50%;
		transform: translateX(-50%);
		font-size: 0.8rem;
		font-family: var(--mono);
		color: #8fa2b9;
		font-weight: 600;
	}

	.latency-chip {
		margin-left: auto;
		font-size: 0.7rem;
		font-family: var(--mono);
		letter-spacing: 0.04em;
		text-transform: uppercase;
		padding: 3px 8px;
		border: 1px solid rgba(255, 255, 255, 0.15);
		background: rgba(15, 17, 26, 0.75);
	}

	.latency-chip.ok {
		color: #86bc25;
	}

	.latency-chip.warn {
		color: #facc15;
	}

	.terminal-scroll-area {
		display: flex;
		flex-direction: column;
		height: calc(72vh - 80px);
		overflow: auto;
		background: transparent;
	}

	.terminal-body {
		margin: 0;
		padding: 16px 16px 8px 16px;
		flex: 1 1 auto;
		font-family: var(--mono);
		font-size: 0.8rem;
		line-height: 1.5;
		color: #e2e8f0;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.terminal-input-area {
		padding: 0 16px 16px 16px;
		flex: 0 0 auto;
	}
</style>
