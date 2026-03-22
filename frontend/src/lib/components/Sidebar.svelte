<script lang="ts">
	import { onMount } from 'svelte';
	import { sidebarOpen } from '$lib/stores';
	import { hasFeatureCapabilityAccess } from '$lib/feature-capability/access';
	import { isFeatureCapabilityEnabled, isTasksEnabled, isTasksKanbanVisible } from '$lib/feature-flags';
	import { page } from '$app/state';
	import { Drawer } from 'flowbite-svelte';
	import { CloseOutline } from 'flowbite-svelte-icons';
	import { shellConfigurationNav, shellRuntimeNav } from '$lib/navigation';

	const canAccessFeatureCapability = $derived(hasFeatureCapabilityAccess());
	const tasksEnabled = $derived(isTasksEnabled());
	const tasksKanbanVisible = $derived(isTasksKanbanVisible());
	const featureCapabilityEnabled = $derived(isFeatureCapabilityEnabled());
	const currentPath = $derived(page.url.pathname);
	const shellNavItems = $derived([...shellRuntimeNav, ...shellConfigurationNav]);

	let desktopExpanded = $state(false);
	let buildLabel = $state('local');

	onMount(() => {
		if (typeof window === 'undefined') {
			return;
		}
		desktopExpanded = window.localStorage.getItem('smith.nav.desktopExpanded') === 'true';
		const config = (window as any).__SMITH_CONFIG__ || {};
		buildLabel = String(config.build || config.version || 'local');
	});

	const systemTag = $derived.by(() => {
		const normalized = String(buildLabel || 'local').trim();
		if (normalized === '') {
			return 'vlocal';
		}
		if (normalized.toLowerCase().startsWith('v')) {
			return normalized;
		}
		return `v${normalized}`;
	});

	function isItemVisible(itemID: string): boolean {
		if (itemID === 'tasks' && !tasksEnabled) {
			return false;
		}
		if (itemID === 'tasks-kanban' && !tasksKanbanVisible) {
			return false;
		}
		if (itemID === 'feature-capability' && (!featureCapabilityEnabled || !canAccessFeatureCapability)) {
			return false;
		}
		return true;
	}

	function isItemActive(href: string): boolean {
		return currentPath.startsWith(href);
	}

	function setDesktopExpanded(next: boolean): void {
		desktopExpanded = next;
		if (typeof window !== 'undefined') {
			window.localStorage.setItem('smith.nav.desktopExpanded', String(next));
		}
	}

	function toggleDesktopExpanded(): void {
		setDesktopExpanded(!desktopExpanded);
	}

	function closeMobileSidebar(): void {
		sidebarOpen.set(false);
	}
</script>

<aside class={`hidden lg:flex nav-rail-shell ${desktopExpanded ? 'expanded' : 'collapsed'}`}>
	<div class="nav-rail-panel">
		<div class="rail-head">
			<button
				type="button"
				class="expand-button"
				onclick={toggleDesktopExpanded}
				aria-label={desktopExpanded ? 'Collapse navigation' : 'Expand navigation'}
			>
				<span class="burger-bars" aria-hidden="true">
					<span></span>
					<span></span>
					<span></span>
				</span>
			</button>
		</div>

		<nav class="nav-list" aria-label="Primary navigation">
			{#each shellNavItems as item}
				{#if isItemVisible(String(item.id))}
					{@const active = isItemActive(item.href)}
					<a
						href={item.href}
						class={`nav-item ${active ? 'active' : ''}`}
						title={desktopExpanded ? '' : item.label}
					>
						<item.icon size="sm" class={`nav-icon ${active ? 'active' : ''}`} />
						{#if desktopExpanded}
							<span class="nav-label">{item.label}</span>
						{/if}
					</a>
				{/if}
			{/each}
		</nav>

		<div class="system-tag">{desktopExpanded ? `System ${systemTag}` : systemTag}</div>
	</div>
</aside>

<Drawer
	bind:open={$sidebarOpen}
	id="sidebar-drawer"
	class="lg:hidden bg-black border-r border-gray-800 p-0 z-50 w-72"
>
	<div class="mobile-shell">
		<div class="mobile-head">
			<div class="mobile-title">Navigation</div>
			<button type="button" class="close-button" onclick={closeMobileSidebar} aria-label="Close Sidebar">
				<CloseOutline size="md" />
			</button>
		</div>

		<div class="mobile-nav-group">
			{#each shellNavItems as item}
				{#if isItemVisible(String(item.id))}
					{@const active = isItemActive(item.href)}
					<a href={item.href} class={`nav-item ${active ? 'active' : ''}`} onclick={closeMobileSidebar}>
						<item.icon size="sm" class={`nav-icon ${active ? 'active' : ''}`} />
						<span class="nav-label">{item.label}</span>
					</a>
				{/if}
			{/each}
		</div>

		<div class="system-tag">System {systemTag}</div>
	</div>
</Drawer>

<style>
	.nav-rail-shell {
		padding-right: 8px;
		transition: width 220ms ease;
	}

	.nav-rail-shell.collapsed {
		width: 74px;
	}

	.nav-rail-shell.expanded {
		width: 232px;
	}

	.nav-rail-panel,
	.mobile-shell {
		display: flex;
		flex-direction: column;
		height: calc(100vh - 2rem);
		background: linear-gradient(180deg, #111420 0%, #0c0f18 58%, #090b12 100%);
		border: 1px solid #1c2230;
		border-radius: 26px;
		box-shadow: 0 18px 40px rgba(0, 0, 0, 0.45);
		padding: 10px;
	}

	.rail-head,
	.mobile-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 8px 6px 12px;
	}

	.mobile-title {
		font-size: 0.78rem;
		font-weight: 800;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: #d9deea;
	}

	.expand-button,
	.close-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 10px;
		border: 1px solid #2a3242;
		background: #0d111b;
		color: #7f8aa1;
	}

	.burger-bars {
		display: inline-flex;
		flex-direction: column;
		gap: 3px;
	}

	.burger-bars span {
		display: block;
		width: 12px;
		height: 1.5px;
		border-radius: 999px;
		background: currentColor;
	}

	.expand-button:hover,
	.close-button:hover {
		color: #e7ebf5;
		border-color: #46536c;
	}

	.nav-list,
	.mobile-nav-group {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.nav-list {
		flex: 1;
		overflow-y: auto;
		padding: 2px 4px 0;
	}

	.mobile-shell {
		padding: 16px;
		height: 100vh;
		border-radius: 0;
		border: 0;
	}

	.mobile-nav-group {
		flex: 1;
		overflow-y: auto;
		padding-top: 2px;
	}

	.nav-item {
		display: inline-flex;
		align-items: center;
		gap: 9px;
		height: 42px;
		width: 100%;
		padding: 0 11px;
		border-radius: 14px;
		border: 1px solid transparent;
		color: #99a2b6;
		background: transparent;
		text-decoration: none;
		font-size: 0.74rem;
		font-weight: 700;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		transition: all 140ms ease;
	}

	.nav-item:hover {
		background: rgba(255, 255, 255, 0.04);
		color: #f3f6ff;
	}

	.nav-item.active {
		background: rgba(255, 255, 255, 0.05);
		border-color: #253142;
		color: #ffffff;
		box-shadow: inset 3px 0 0 #86bc25;
	}

	.nav-rail-shell.collapsed .nav-item {
		justify-content: center;
		padding: 0;
		width: 42px;
		margin: 0 auto;
	}

	.nav-rail-shell.collapsed .expand-button {
		margin: 0 auto;
	}

	.nav-rail-shell.collapsed .rail-head {
		justify-content: center;
		padding-left: 0;
		padding-right: 0;
	}

	.nav-rail-shell.collapsed .nav-list {
		padding-left: 0;
		padding-right: 0;
	}

	.nav-rail-shell.collapsed .nav-item.active {
		box-shadow: inset 0 0 0 1px rgba(134, 188, 37, 0.5);
	}

	.nav-icon {
		color: #7d8699;
	}

	.nav-icon.active,
	.nav-icon.chat {
		color: #86bc25;
	}

	.nav-label {
		white-space: nowrap;
	}

	.system-tag {
		margin-top: auto;
		padding: 10px;
		font-size: 0.62rem;
		text-transform: uppercase;
		letter-spacing: 0.16em;
		font-weight: 700;
		color: #62708b;
	}

	:global(#sidebar-drawer) {
		background-color: #000000 !important;
	}
</style>
