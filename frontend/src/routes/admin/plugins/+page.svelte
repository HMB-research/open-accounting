<script lang="ts">
	import {
		api,
		type Plugin,
		type PluginRegistry,
		type PluginSearchResult,
		type PluginPermission,
		type PermissionRisk
	} from '$lib/api';

	let plugins = $state<Plugin[]>([]);
	let registries = $state<PluginRegistry[]>([]);
	let searchResults = $state<PluginSearchResult[]>([]);
	let allPermissions = $state<Record<string, PluginPermission>>({});

	let isLoading = $state(true);
	let error = $state('');
	let successMessage = $state('');

	// Modal states
	let showAddRegistry = $state(false);
	let showInstallPlugin = $state(false);
	let showEnablePlugin = $state(false);
	let showSearch = $state(false);

	// Form states
	let newRegistryName = $state('');
	let newRegistryUrl = $state('');
	let newRegistryDescription = $state('');
	let installUrl = $state('');
	let searchQuery = $state('');
	let selectedPlugin = $state<Plugin | null>(null);
	let selectedPermissions = $state<string[]>([]);

	// Tab state
	let activeTab = $state<'installed' | 'registries'>('installed');

	$effect(() => {
		loadData();
	});

	async function loadData() {
		isLoading = true;
		error = '';

		try {
			const [pluginsData, registriesData, permissionsData] = await Promise.all([
				api.listPlugins(),
				api.listPluginRegistries(),
				api.getPluginPermissions()
			]);
			plugins = pluginsData;
			registries = registriesData;
			allPermissions = permissionsData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	async function addRegistry(e: Event) {
		e.preventDefault();
		error = '';

		try {
			const registry = await api.addPluginRegistry(
				newRegistryName,
				newRegistryUrl,
				newRegistryDescription || undefined
			);
			registries = [...registries, registry];
			showAddRegistry = false;
			resetRegistryForm();
			successMessage = 'Registry added successfully';
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to add registry';
		}
	}

	function resetRegistryForm() {
		newRegistryName = '';
		newRegistryUrl = '';
		newRegistryDescription = '';
	}

	async function removeRegistry(registryId: string) {
		if (!confirm('Are you sure you want to remove this registry?')) return;

		try {
			await api.removePluginRegistry(registryId);
			registries = registries.filter((r) => r.id !== registryId);
			successMessage = 'Registry removed';
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to remove registry';
		}
	}

	async function syncRegistry(registryId: string) {
		try {
			await api.syncPluginRegistry(registryId);
			successMessage = 'Registry synced successfully';
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to sync registry';
		}
	}

	async function installPlugin(e: Event) {
		e.preventDefault();
		error = '';

		try {
			const plugin = await api.installPlugin(installUrl);
			plugins = [...plugins, plugin];
			showInstallPlugin = false;
			installUrl = '';
			successMessage = `Plugin "${plugin.display_name}" installed successfully`;
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to install plugin';
		}
	}

	async function uninstallPlugin(pluginId: string, pluginName: string) {
		if (!confirm(`Are you sure you want to uninstall "${pluginName}"? This will remove all plugin data.`))
			return;

		try {
			await api.uninstallPlugin(pluginId);
			plugins = plugins.filter((p) => p.id !== pluginId);
			successMessage = `Plugin "${pluginName}" uninstalled`;
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to uninstall plugin';
		}
	}

	function openEnableModal(plugin: Plugin) {
		selectedPlugin = plugin;
		selectedPermissions = [...plugin.manifest.permissions];
		showEnablePlugin = true;
	}

	async function enablePlugin() {
		if (!selectedPlugin) return;

		try {
			const updated = await api.enablePlugin(selectedPlugin.id, selectedPermissions);
			plugins = plugins.map((p) => (p.id === updated.id ? updated : p));
			showEnablePlugin = false;
			selectedPlugin = null;
			successMessage = `Plugin "${updated.display_name}" enabled`;
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to enable plugin';
		}
	}

	async function disablePlugin(pluginId: string) {
		try {
			const updated = await api.disablePlugin(pluginId);
			plugins = plugins.map((p) => (p.id === updated.id ? updated : p));
			successMessage = `Plugin "${updated.display_name}" disabled`;
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to disable plugin';
		}
	}

	async function searchPlugins() {
		if (!searchQuery.trim()) return;

		try {
			searchResults = await api.searchPlugins(searchQuery);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Search failed';
		}
	}

	async function installFromSearch(result: PluginSearchResult) {
		try {
			const plugin = await api.installPlugin(result.plugin.repository);
			plugins = [...plugins, plugin];
			searchResults = searchResults.filter((r) => r.plugin.repository !== result.plugin.repository);
			successMessage = `Plugin "${plugin.display_name}" installed`;
			setTimeout(() => (successMessage = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to install plugin';
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

	function getStateClass(state: string): string {
		switch (state) {
			case 'enabled':
				return 'state-enabled';
			case 'disabled':
				return 'state-disabled';
			case 'installed':
				return 'state-installed';
			case 'failed':
				return 'state-failed';
			default:
				return '';
		}
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	function isPluginInstalled(repository: string): boolean {
		return plugins.some((p) => p.repository_url === repository);
	}
</script>

<svelte:head>
	<title>Plugin Marketplace - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<div>
			<h1>Plugin Marketplace</h1>
			<p class="subtitle">Manage plugins and registries for your Open Accounting instance</p>
		</div>
		<div class="header-actions">
			<button class="btn btn-secondary" onclick={() => (showSearch = true)}>Search Plugins</button>
			<button class="btn btn-primary" onclick={() => (showInstallPlugin = true)}>
				+ Install from URL
			</button>
		</div>
	</div>

	{#if successMessage}
		<div class="alert alert-success">{successMessage}</div>
	{/if}

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	<div class="tabs">
		<button
			class="tab-btn"
			class:active={activeTab === 'installed'}
			onclick={() => (activeTab = 'installed')}
		>
			Installed Plugins ({plugins.length})
		</button>
		<button
			class="tab-btn"
			class:active={activeTab === 'registries'}
			onclick={() => (activeTab = 'registries')}
		>
			Registries ({registries.length})
		</button>
	</div>

	{#if isLoading}
		<div class="loading">Loading...</div>
	{:else if activeTab === 'installed'}
		{#if plugins.length === 0}
			<div class="empty-state card">
				<h3>No plugins installed</h3>
				<p>Install plugins from registries or directly from GitHub/GitLab repositories.</p>
				<button class="btn btn-primary" onclick={() => (showInstallPlugin = true)}>
					Install Your First Plugin
				</button>
			</div>
		{:else}
			<div class="plugins-grid">
				{#each plugins as plugin}
					<div class="plugin-card card">
						<div class="plugin-header">
							<div class="plugin-info">
								<h3>{plugin.display_name}</h3>
								<span class="badge {getStateClass(plugin.state)}">{plugin.state}</span>
							</div>
							<span class="version">v{plugin.version}</span>
						</div>

						{#if plugin.description}
							<p class="description">{plugin.description}</p>
						{/if}

						<div class="plugin-meta">
							{#if plugin.author}
								<span class="meta-item">By {plugin.author}</span>
							{/if}
							{#if plugin.license}
								<span class="meta-item">{plugin.license}</span>
							{/if}
							<span class="meta-item">Installed {formatDate(plugin.installed_at)}</span>
						</div>

						{#if plugin.granted_permissions.length > 0}
							<div class="permissions-summary">
								<span class="permissions-label">Permissions:</span>
								<div class="permission-badges">
									{#each plugin.granted_permissions.slice(0, 3) as perm}
										{@const permInfo = allPermissions[perm]}
										<span
											class="permission-badge {permInfo ? getPermissionRiskClass(permInfo.risk) : ''}"
											title={permInfo?.description || perm}
										>
											{perm}
										</span>
									{/each}
									{#if plugin.granted_permissions.length > 3}
										<span class="permission-badge more">
											+{plugin.granted_permissions.length - 3} more
										</span>
									{/if}
								</div>
							</div>
						{/if}

						<div class="plugin-actions">
							{#if plugin.state === 'enabled'}
								<button class="btn btn-sm btn-secondary" onclick={() => disablePlugin(plugin.id)}>
									Disable
								</button>
							{:else if plugin.state === 'installed' || plugin.state === 'disabled'}
								<button class="btn btn-sm btn-primary" onclick={() => openEnableModal(plugin)}>
									Enable
								</button>
							{/if}
							<button
								class="btn btn-sm btn-danger"
								onclick={() => uninstallPlugin(plugin.id, plugin.display_name)}
							>
								Uninstall
							</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{:else}
		<div class="registries-header">
			<button class="btn btn-primary" onclick={() => (showAddRegistry = true)}>
				+ Add Registry
			</button>
		</div>

		{#if registries.length === 0}
			<div class="empty-state card">
				<h3>No registries configured</h3>
				<p>Add plugin registries to discover and install community plugins.</p>
			</div>
		{:else}
			<div class="registries-list">
				{#each registries as registry}
					<div class="registry-card card">
						<div class="registry-header">
							<div class="registry-info">
								<h3>
									{registry.name}
									{#if registry.is_official}
										<span class="badge badge-official">Official</span>
									{/if}
								</h3>
								<a href={registry.url} target="_blank" rel="noopener" class="registry-url">
									{registry.url}
								</a>
							</div>
							<span class="badge {registry.is_active ? 'badge-active' : 'badge-inactive'}">
								{registry.is_active ? 'Active' : 'Inactive'}
							</span>
						</div>

						{#if registry.description}
							<p class="description">{registry.description}</p>
						{/if}

						<div class="registry-meta">
							{#if registry.last_synced_at}
								<span>Last synced: {formatDate(registry.last_synced_at)}</span>
							{:else}
								<span>Never synced</span>
							{/if}
						</div>

						<div class="registry-actions">
							<button class="btn btn-sm btn-secondary" onclick={() => syncRegistry(registry.id)}>
								Sync Now
							</button>
							{#if !registry.is_official}
								<button
									class="btn btn-sm btn-danger"
									onclick={() => removeRegistry(registry.id)}
								>
									Remove
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{/if}
</div>

<!-- Add Registry Modal -->
{#if showAddRegistry}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showAddRegistry = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<h2>Add Plugin Registry</h2>
			<form onsubmit={addRegistry}>
				<div class="form-group">
					<label class="label" for="registryName">Registry Name *</label>
					<input
						class="input"
						type="text"
						id="registryName"
						bind:value={newRegistryName}
						required
						placeholder="Community Plugins"
					/>
				</div>

				<div class="form-group">
					<label class="label" for="registryUrl">Repository URL *</label>
					<input
						class="input"
						type="url"
						id="registryUrl"
						bind:value={newRegistryUrl}
						required
						placeholder="https://github.com/owner/plugins-registry"
					/>
					<small class="help-text">GitHub or GitLab repository URL containing plugins.yaml</small>
				</div>

				<div class="form-group">
					<label class="label" for="registryDescription">Description</label>
					<textarea
						class="input"
						id="registryDescription"
						bind:value={newRegistryDescription}
						placeholder="Optional description..."
						rows="2"
					></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showAddRegistry = false)}>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Add Registry</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Install Plugin Modal -->
{#if showInstallPlugin}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showInstallPlugin = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<h2>Install Plugin from URL</h2>
			<form onsubmit={installPlugin}>
				<div class="form-group">
					<label class="label" for="installUrl">Repository URL *</label>
					<input
						class="input"
						type="url"
						id="installUrl"
						bind:value={installUrl}
						required
						placeholder="https://github.com/owner/plugin-name"
					/>
					<small class="help-text">
						Enter the GitHub or GitLab repository URL. The repository must contain a valid
						plugin.yaml manifest and a LICENSE file.
					</small>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showInstallPlugin = false)}
					>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Install Plugin</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Enable Plugin Modal -->
{#if showEnablePlugin && selectedPlugin}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showEnablePlugin = false)} role="presentation">
		<div
			class="modal card modal-lg"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<h2>Enable Plugin: {selectedPlugin.display_name}</h2>

			<div class="permission-review">
				<h3>Review Permissions</h3>
				<p class="permission-notice">
					This plugin requires the following permissions. Review each carefully before enabling.
				</p>

				<div class="permissions-list">
					{#each selectedPlugin.manifest.permissions as perm}
						{@const permInfo = allPermissions[perm]}
						<div class="permission-item">
							<label class="checkbox-label">
								<input
									type="checkbox"
									checked={selectedPermissions.includes(perm)}
									onchange={(e) => {
										if (e.currentTarget.checked) {
											selectedPermissions = [...selectedPermissions, perm];
										} else {
											selectedPermissions = selectedPermissions.filter((p) => p !== perm);
										}
									}}
								/>
								<div class="permission-details">
									<div class="permission-name">
										{perm}
										{#if permInfo}
											<span class="risk-badge {getPermissionRiskClass(permInfo.risk)}">
												{permInfo.risk}
											</span>
										{/if}
									</div>
									{#if permInfo}
										<div class="permission-desc">{permInfo.description}</div>
									{/if}
								</div>
							</label>
						</div>
					{/each}
				</div>

				{#if selectedPlugin.manifest.permissions.some((p) => {
					const info = allPermissions[p];
					return info && (info.risk === 'high' || info.risk === 'critical');
				})}
					<div class="warning-box">
						<strong>Warning:</strong> This plugin requests high-risk permissions. Only enable if you
						trust the source.
					</div>
				{/if}
			</div>

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (showEnablePlugin = false)}>
					Cancel
				</button>
				<button
					type="button"
					class="btn btn-primary"
					onclick={enablePlugin}
					disabled={selectedPermissions.length === 0}
				>
					Enable with {selectedPermissions.length} Permissions
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Search Modal -->
{#if showSearch}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showSearch = false)} role="presentation">
		<div
			class="modal card modal-lg"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<h2>Search Plugins</h2>

			<div class="search-form">
				<div class="search-row">
					<input
						class="input"
						type="text"
						bind:value={searchQuery}
						placeholder="Search plugins by name, description, or tags..."
						onkeydown={(e) => e.key === 'Enter' && searchPlugins()}
					/>
					<button class="btn btn-primary" onclick={searchPlugins}>Search</button>
				</div>
			</div>

			{#if searchResults.length > 0}
				<div class="search-results">
					{#each searchResults as result}
						<div class="search-result-item">
							<div class="result-info">
								<h4>{result.plugin.display_name}</h4>
								<span class="result-registry">from {result.registry}</span>
								{#if result.plugin.description}
									<p class="result-desc">{result.plugin.description}</p>
								{/if}
								<div class="result-meta">
									{#if result.plugin.author}
										<span>By {result.plugin.author}</span>
									{/if}
									<span>v{result.plugin.version}</span>
									{#if result.plugin.license}
										<span>{result.plugin.license}</span>
									{/if}
								</div>
								{#if result.plugin.tags && result.plugin.tags.length > 0}
									<div class="result-tags">
										{#each result.plugin.tags as tag}
											<span class="tag">{tag}</span>
										{/each}
									</div>
								{/if}
							</div>
							<div class="result-actions">
								{#if isPluginInstalled(result.plugin.repository)}
									<span class="installed-badge">Installed</span>
								{:else}
									<button
										class="btn btn-sm btn-primary"
										onclick={() => installFromSearch(result)}
									>
										Install
									</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{:else if searchQuery}
				<div class="no-results">
					<p>No plugins found matching "{searchQuery}"</p>
				</div>
			{:else}
				<div class="search-hint">
					<p>Enter a search term to find plugins across all registered marketplaces.</p>
				</div>
			{/if}

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (showSearch = false)}>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
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

	.header-actions {
		display: flex;
		gap: 0.5rem;
	}

	.tabs {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 1.5rem;
		border-bottom: 2px solid var(--color-border);
	}

	.tab-btn {
		padding: 0.75rem 1.5rem;
		background: none;
		border: none;
		font-weight: 500;
		color: var(--color-text-muted);
		cursor: pointer;
		margin-bottom: -2px;
		border-bottom: 2px solid transparent;
	}

	.tab-btn:hover {
		color: var(--color-text);
	}

	.tab-btn.active {
		color: var(--color-primary);
		border-bottom-color: var(--color-primary);
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
		margin-bottom: 1.5rem;
	}

	.plugins-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
		gap: 1rem;
	}

	.plugin-card {
		display: flex;
		flex-direction: column;
	}

	.plugin-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 0.75rem;
	}

	.plugin-info {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.plugin-info h3 {
		font-size: 1.1rem;
		margin: 0;
	}

	.version {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.description {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		margin-bottom: 0.75rem;
		flex-grow: 1;
	}

	.plugin-meta {
		display: flex;
		gap: 1rem;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-bottom: 0.75rem;
	}

	.permissions-summary {
		margin-bottom: 1rem;
	}

	.permissions-label {
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

	.permission-badge.more {
		background: transparent;
		color: var(--color-text-muted);
	}

	.plugin-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: auto;
		padding-top: 0.75rem;
		border-top: 1px solid var(--color-border);
	}

	/* State badges */
	.state-enabled {
		background: #dcfce7;
		color: #166534;
	}

	.state-disabled {
		background: #fef2f2;
		color: #991b1b;
	}

	.state-installed {
		background: #fef3c7;
		color: #92400e;
	}

	.state-failed {
		background: #fee2e2;
		color: #b91c1c;
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

	/* Registry styles */
	.registries-header {
		margin-bottom: 1rem;
	}

	.registries-list {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.registry-card {
		display: flex;
		flex-direction: column;
	}

	.registry-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 0.5rem;
	}

	.registry-info h3 {
		margin: 0;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.registry-url {
		font-size: 0.875rem;
		color: var(--color-primary);
	}

	.registry-meta {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-bottom: 0.75rem;
	}

	.registry-actions {
		display: flex;
		gap: 0.5rem;
	}

	.badge-official {
		background: #e0e7ff;
		color: #3730a3;
	}

	.badge-active {
		background: #dcfce7;
		color: #166534;
	}

	.badge-inactive {
		background: #f3f4f6;
		color: #6b7280;
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

	.modal-lg {
		max-width: 700px;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.help-text {
		display: block;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}

	/* Permission review */
	.permission-review h3 {
		font-size: 1rem;
		margin-bottom: 0.5rem;
	}

	.permission-notice {
		color: var(--color-text-muted);
		margin-bottom: 1rem;
	}

	.permissions-list {
		border: 1px solid var(--color-border);
		border-radius: 4px;
		overflow: hidden;
	}

	.permission-item {
		padding: 0.75rem 1rem;
		border-bottom: 1px solid var(--color-border);
	}

	.permission-item:last-child {
		border-bottom: none;
	}

	.permission-item .checkbox-label {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		cursor: pointer;
	}

	.permission-item input[type='checkbox'] {
		margin-top: 0.25rem;
	}

	.permission-details {
		flex: 1;
	}

	.permission-name {
		font-weight: 500;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.risk-badge {
		font-size: 0.65rem;
		padding: 0.125rem 0.375rem;
		border-radius: 9999px;
		text-transform: uppercase;
		font-weight: 600;
	}

	.permission-desc {
		font-size: 0.8rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	.warning-box {
		margin-top: 1rem;
		padding: 0.75rem 1rem;
		background: #fef3c7;
		border: 1px solid #fcd34d;
		border-radius: 4px;
		color: #92400e;
		font-size: 0.875rem;
	}

	/* Search styles */
	.search-form {
		margin-bottom: 1.5rem;
	}

	.search-row {
		display: flex;
		gap: 0.5rem;
	}

	.search-row .input {
		flex: 1;
	}

	.search-results {
		max-height: 400px;
		overflow-y: auto;
	}

	.search-result-item {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		margin-bottom: 0.5rem;
	}

	.result-info h4 {
		margin: 0 0 0.25rem;
	}

	.result-registry {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.result-desc {
		font-size: 0.875rem;
		color: var(--color-text-muted);
		margin: 0.5rem 0;
	}

	.result-meta {
		display: flex;
		gap: 1rem;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.result-tags {
		display: flex;
		gap: 0.25rem;
		margin-top: 0.5rem;
	}

	.tag {
		font-size: 0.7rem;
		padding: 0.125rem 0.5rem;
		background: var(--color-bg);
		border-radius: 9999px;
	}

	.installed-badge {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.no-results,
	.search-hint {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
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
