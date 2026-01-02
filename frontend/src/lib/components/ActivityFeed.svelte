<script lang="ts">
	import * as m from '$lib/paraglide/messages.js';

	export interface ActivityItem {
		id: string;
		type: 'INVOICE' | 'PAYMENT' | 'ENTRY' | 'CONTACT';
		action: string;
		description: string;
		amount?: string;
		created_at: string;
	}

	interface Props {
		items: ActivityItem[];
		loading?: boolean;
	}

	let { items = [], loading = false }: Props = $props();

	function getIcon(type: string): string {
		switch (type) {
			case 'INVOICE':
				return '📄';
			case 'PAYMENT':
				return '💰';
			case 'ENTRY':
				return '📝';
			case 'CONTACT':
				return '👤';
			default:
				return '📌';
		}
	}

	function formatTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return date.toLocaleDateString();
	}

	function formatAmount(amount: string | undefined): string {
		if (!amount) return '';
		const num = parseFloat(amount);
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR'
		}).format(num);
	}
</script>

<div class="activity-feed" data-testid="activity-feed">
	<h3 class="feed-title">{m.dashboard_recentActivity()}</h3>

	{#if loading}
		<div class="loading">
			<div class="spinner"></div>
		</div>
	{:else if items.length === 0}
		<p class="empty">{m.dashboard_noRecentActivity()}</p>
	{:else}
		<ul class="activity-list">
			{#each items as item}
				<li class="activity-item" data-testid="activity-item">
					<span class="activity-icon">{getIcon(item.type)}</span>
					<div class="activity-content">
						<p class="activity-description">{item.description}</p>
						<div class="activity-meta">
							<span class="activity-time">{formatTime(item.created_at)}</span>
							{#if item.amount}
								<span class="activity-amount">{formatAmount(item.amount)}</span>
							{/if}
						</div>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.activity-feed {
		background: var(--color-card);
		border-radius: var(--radius-md);
		border: 1px solid var(--color-border);
		padding: 1rem;
	}

	.feed-title {
		font-size: 1rem;
		font-weight: 600;
		margin-bottom: 1rem;
		color: var(--color-text);
	}

	.activity-list {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.activity-item {
		display: flex;
		gap: 0.75rem;
		padding: 0.75rem 0;
		border-bottom: 1px solid var(--color-border);
	}

	.activity-item:last-child {
		border-bottom: none;
		padding-bottom: 0;
	}

	.activity-item:first-child {
		padding-top: 0;
	}

	.activity-icon {
		font-size: 1.25rem;
		flex-shrink: 0;
	}

	.activity-content {
		flex: 1;
		min-width: 0;
	}

	.activity-description {
		margin: 0;
		font-size: 0.875rem;
		color: var(--color-text);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.activity-meta {
		display: flex;
		justify-content: space-between;
		margin-top: 0.25rem;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.activity-amount {
		font-weight: 500;
		color: #22c55e;
	}

	.loading {
		display: flex;
		justify-content: center;
		padding: 2rem;
	}

	.spinner {
		width: 1.5rem;
		height: 1.5rem;
		border: 2px solid var(--color-border);
		border-top-color: var(--color-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 2rem;
		margin: 0;
	}
</style>
