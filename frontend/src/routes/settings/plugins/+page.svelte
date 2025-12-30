<script lang="ts">
	import { page } from '$app/stores';
	import {
		api,
		type TenantPlugin,
		type Plugin,
		type PluginPermission,
		type PermissionRisk
	} from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let tenantPlugins = $state<TenantPlugin[]>([]);
	let allPermissions = $state<Record<string, PluginPermission>>({});

	let isLoading = $state(true);
	let error = $state('');
	let successMessage = $state('');

	// Modal states
	let showSettings = $state(false);
	let selectedPlugin = $state<TenantPlugin | null>(null);
	let editedSettings = $state<Record<string, unknown>>({});

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	});

	async function loadData(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			const [pluginsData, permissionsData] = await Promise.all([
				api.listTenantPlugins(tenantId),
				api.getPluginPermissions()
			]);
			tenantPlugins = pluginsData;
			allPermissions = permissionsData;
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		} finally {
			isLoading = false;
		}
	}

	async function enablePlugin(pluginId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const updated = await api.enableTenantPlugin(tenantId, pluginId);
			tenantPlugins = tenantPlugins.map((p) => (p.plugin_id === updated.plugin_id ? updated : p));
			successMessage = m.plugins_enabledSuccess();
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		}
	}

	async function disablePlugin(pluginId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!confirm(m.plugins_confirmDisable())) return;

		try {
			await api.disableTenantPlugin(tenantId, pluginId);
			tenantPlugins = tenantPlugins.map((p) =>
				p.plugin_id === pluginId ? { ...p, is_enabled: false, enabled_at: undefined } : p
			);
			successMessage = m.plugins_disabledSuccess();
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		}
	}

	async function openSettingsModal(plugin: TenantPlugin) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const settingsData = await api.getTenantPluginSettings(tenantId, plugin.plugin_id);
			selectedPlugin = plugin;
			editedSettings = { ...settingsData.settings };
			showSettings = true;
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		}
	}

	async function saveSettings() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId || !selectedPlugin) return;

		try {
			await api.updateTenantPluginSettings(tenantId, selectedPlugin.plugin_id, editedSettings);
			tenantPlugins = tenantPlugins.map((p) =>
				p.plugin_id === selectedPlugin!.plugin_id ? { ...p, settings: editedSettings } : p
			);
			showSettings = false;
			selectedPlugin = null;
			successMessage = m.plugins_settingsSaved();
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		}
	}

	function getPermissionRiskClass(risk: PermissionRisk): string {
		switch (risk) {
			case 'low':
				return 'risk-low';
			case 'medium':
				return 'risk-medium';
			case 'high':
				return 'risk-high';
			case 'critical':
				return 'risk-critical';
			default:
				return '';
		}
	}

	function formatDate(dateStr: string | undefined): string {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	function hasSettings(plugin: TenantPlugin): boolean {
		return (
			plugin.plugin?.manifest?.settings !== undefined &&
			Object.keys(plugin.plugin?.manifest?.settings || {}).length > 0
		);
	}
</script>

<svelte:head>
	<title>{m.plugins_pluginSettings()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<div>
			<h1>{m.plugins_pluginSettings()}</h1>
			<p class="subtitle">{m.plugins_managePlugins()}</p>
		</div>
	</div>

	{#if successMessage}
		<div class="alert alert-success">{successMessage}</div>
	{/if}

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<div class="loading">{m.plugins_loadingPlugins()}</div>
	{:else if tenantPlugins.length === 0}
		<div class="empty-state card">
			<h3>{m.plugins_noPlugins()}</h3>
			<p>{m.plugins_noPluginsDesc()}</p>
		</div>
	{:else}
		<div class="plugins-list">
			{#each tenantPlugins as tenantPlugin}
				{@const plugin = tenantPlugin.plugin}
				{#if plugin}
					<div class="plugin-card card" class:enabled={tenantPlugin.is_enabled}>
						<div class="plugin-header">
							<div class="plugin-info">
								<h3>{plugin.display_name}</h3>
								<span class="version">v{plugin.version}</span>
							</div>
							<span class="badge" class:badge-enabled={tenantPlugin.is_enabled}>
								{tenantPlugin.is_enabled ? m.plugins_enabled() : m.plugins_disabled()}
							</span>
						</div>

						{#if plugin.description}
							<p class="description">{plugin.description}</p>
						{/if}

						{#if plugin.granted_permissions.length > 0}
							<div class="permissions-section">
								<span class="section-label">{m.plugins_grantedPermissions()}</span>
								<div class="permission-badges">
									{#each plugin.granted_permissions as perm}
										{@const permInfo = allPermissions[perm]}
										<span
											class="permission-badge {permInfo ? getPermissionRiskClass(permInfo.risk) : ''}"
											title={permInfo?.description || perm}
										>
											{perm}
										</span>
									{/each}
								</div>
							</div>
						{/if}

						{#if tenantPlugin.is_enabled && tenantPlugin.enabled_at}
							<div class="plugin-meta">
								<span>{m.plugins_enabledOn()} {formatDate(tenantPlugin.enabled_at)}</span>
							</div>
						{/if}

						<div class="plugin-actions">
							{#if tenantPlugin.is_enabled}
								{#if hasSettings(tenantPlugin)}
									<button
										class="btn btn-sm btn-secondary"
										onclick={() => openSettingsModal(tenantPlugin)}
									>
										{m.plugins_settings()}
									</button>
								{/if}
								<button
									class="btn btn-sm btn-danger"
									onclick={() => disablePlugin(tenantPlugin.plugin_id)}
								>
									{m.plugins_disable()}
								</button>
							{:else}
								<button
									class="btn btn-sm btn-primary"
									onclick={() => enablePlugin(tenantPlugin.plugin_id)}
								>
									{m.plugins_enable()}
								</button>
							{/if}
						</div>
					</div>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<!-- Settings Modal -->
{#if showSettings && selectedPlugin}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showSettings = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<h2>{selectedPlugin.plugin?.display_name} Settings</h2>

			<div class="settings-form">
				{#if selectedPlugin.plugin?.manifest?.settings}
					{@const schema = selectedPlugin.plugin.manifest.settings as Record<string, unknown>}
					{@const properties = (schema.properties || {}) as Record<
						string,
						{ type?: string; default?: unknown; description?: string }
					>}

					{#each Object.entries(properties) as [key, prop]}
						<div class="form-group">
							<label class="label" for={key}>{key}</label>

							{#if prop.type === 'boolean'}
								<label class="checkbox-label">
									<input
										type="checkbox"
										id={key}
										checked={editedSettings[key] as boolean ??
											(prop.default as boolean) ??
											false}
										onchange={(e) => (editedSettings[key] = e.currentTarget.checked)}
									/>
									{prop.description || ''}
								</label>
							{:else if prop.type === 'number'}
								<input
									class="input"
									type="number"
									id={key}
									value={editedSettings[key] as number ?? (prop.default as number) ?? 0}
									oninput={(e) => (editedSettings[key] = Number(e.currentTarget.value))}
								/>
								{#if prop.description}
									<small class="help-text">{prop.description}</small>
								{/if}
							{:else}
								<input
									class="input"
									type="text"
									id={key}
									value={editedSettings[key] as string ?? (prop.default as string) ?? ''}
									oninput={(e) => (editedSettings[key] = e.currentTarget.value)}
								/>
								{#if prop.description}
									<small class="help-text">{prop.description}</small>
								{/if}
							{/if}
						</div>
					{/each}
				{:else}
					<p class="no-settings">{m.plugins_noSettings()}</p>
				{/if}
			</div>

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (showSettings = false)}>
					{m.common_cancel()}
				</button>
				<button type="button" class="btn btn-primary" onclick={saveSettings}>{m.plugins_saveSettings()}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.header {
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
		margin-bottom: 0.25rem;
	}

	.subtitle {
		color: var(--color-text-muted);
		margin: 0;
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
	}

	.empty-state h3 {
		margin-bottom: 0.5rem;
	}

	.empty-state p {
		color: var(--color-text-muted);
	}

	.plugins-list {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.plugin-card {
		display: flex;
		flex-direction: column;
		opacity: 0.7;
		transition: opacity 0.2s;
	}

	.plugin-card.enabled {
		opacity: 1;
		border-left: 3px solid var(--color-primary);
	}

	.plugin-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 0.5rem;
	}

	.plugin-info {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.plugin-info h3 {
		margin: 0;
		font-size: 1.1rem;
	}

	.version {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.description {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		margin-bottom: 0.75rem;
	}

	.permissions-section {
		margin-bottom: 0.75rem;
	}

	.section-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		display: block;
		margin-bottom: 0.25rem;
	}

	.permission-badges {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
	}

	.permission-badge {
		font-size: 0.7rem;
		padding: 0.125rem 0.5rem;
		border-radius: 9999px;
		background: var(--color-bg);
		border: 1px solid var(--color-border);
	}

	.plugin-meta {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-bottom: 0.75rem;
	}

	.plugin-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: auto;
		padding-top: 0.75rem;
		border-top: 1px solid var(--color-border);
	}

	.badge {
		padding: 0.25rem 0.75rem;
		border-radius: 9999px;
		font-size: 0.75rem;
		font-weight: 500;
		background: #f3f4f6;
		color: #6b7280;
	}

	.badge-enabled {
		background: #dcfce7;
		color: #166534;
	}

	/* Risk colors */
	.risk-low {
		background: #dcfce7;
		color: #166534;
		border-color: #86efac;
	}

	.risk-medium {
		background: #fef3c7;
		color: #92400e;
		border-color: #fcd34d;
	}

	.risk-high {
		background: #fed7aa;
		color: #9a3412;
		border-color: #fdba74;
	}

	.risk-critical {
		background: #fee2e2;
		color: #b91c1c;
		border-color: #fca5a5;
	}

	/* Modal styles */
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		width: 100%;
		max-width: 500px;
		margin: 1rem;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.settings-form {
		margin-bottom: 1rem;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	.help-text {
		display: block;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	.no-settings {
		color: var(--color-text-muted);
		text-align: center;
		padding: 1rem;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}

	.btn-sm {
		padding: 0.375rem 0.75rem;
		font-size: 0.875rem;
	}

	.btn-danger {
		background: #ef4444;
		color: white;
	}

	.btn-danger:hover {
		background: #dc2626;
	}

	.alert-success {
		background: #dcfce7;
		color: #166534;
		border: 1px solid #86efac;
		padding: 0.75rem 1rem;
		border-radius: 4px;
		margin-bottom: 1rem;
	}
</style>
